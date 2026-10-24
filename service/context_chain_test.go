package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fireflycore/go-micro/authz"
	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc/metadata"
)

// chainAuthorityProvider 为当前调用服务提供固定测试凭证，不模拟身份认证或在线 session。
type chainAuthorityProvider string

func (p chainAuthorityProvider) ServiceAuthority(context.Context) (string, error) {
	return string(p), nil
}

// chainClaims 仅模拟每跳 authz 重新签发，实际入口始终由测试中的本地适配事实提供。
func chainClaims(subject string, user bool, invokeApp, targetApp, path string) map[string]any {
	claims := authzTestClaims(subject, user)
	claims["service_app_id"], claims["service_path"] = targetApp, path
	if subject == constant.SubjectTypeService {
		claims["sub"], claims["subject_app_id"], claims["invoke_service_app_id"] = invokeApp, invokeApp, invokeApp
	}
	return claims
}

// chainEntry 建立新一跳入口，不能继承上一服务的私有 context 值或本地身份。
func chainEntry(t *testing.T, md metadata.MD, app, path string) context.Context {
	t.Helper()
	verification := authzTestOptions()
	ctx, err := service.NewIncomingContext(context.Background(), md, service.BuildContextOptions{
		ServiceAppId: app, ServiceInstanceId: app + "-instance",
		ServiceMethod: constant.RequestMethodGrpcString, ServicePath: path,
		AuthzVerification: &verification,
	})
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

// TestContextChainKeepsUserAndReplacesEachHop 独立于 Python 验证 A→B→C 的签名、凭证和观测边界。
// 本测试仅覆盖框架协议，不代表真实 authz 已验证 token、在线 session 或 API 权限。
func TestContextChainKeepsUserAndReplacesEachHop(t *testing.T) {
	for _, tc := range []struct {
		name    string // 四种原始入口分别验证，不用服务身份覆盖用户视图。
		subject string // A 入口签名的授权主体类型。
		user    bool   // 是否应贯穿整条链保留独立用户身份及私有凭证。
	}{
		{name: "anonymous", subject: constant.SubjectTypeAnonymous},
		{name: "user", subject: constant.SubjectTypeUser, user: true},
		{name: "service", subject: constant.SubjectTypeService},
		{name: "dual_token", subject: constant.SubjectTypeService, user: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const aPath, bPath, cPath = "/a.Service/Run", "/b.Service/Run", "/c.Service/Run"
			proofA := authzTestSign(t, chainClaims(tc.subject, tc.user, "origin-service", "A", aPath))
			incoming := metadata.Pairs(constant.AuthzSign, proofA, constant.InvokeMethod, "POST", constant.InvokePath, "/origin")
			userAuthority := ""
			if tc.user {
				userAuthority = "original-user-token"
				incoming.Set(constant.UserAuthority, userAuthority)
			}
			ctxA := chainEntry(t, incoming, "A", aPath)
			initial := service.MustFromContext(ctxA)
			if (initial.UserContext != nil) != tc.user {
				t.Fatal("A lost initial user presence")
			}
			if tc.subject == constant.SubjectTypeService {
				if initial.InvokeAppId != "origin-service" || initial.InvokePath != "/origin" {
					t.Fatal("A lost authenticated previous service")
				}
			} else if initial.InvokeAppId != "" || initial.InvokeMethod != "" || initial.InvokePath != "" {
				t.Fatal("direct user or anonymous input forged an Invoke entry")
			}

			// A 的下一跳必须使用 A 的 provider，不继承 A 入口的授权证明或普通身份头。
			toB, err := authz.PrepareOutgoingAuthorityMetadata(ctxA, service.OutgoingTarget{
				TargetAppId: "B", TargetMethod: constant.RequestMethodGrpcString, TargetPath: bPath,
			}, chainAuthorityProvider("A-token"))
			if err != nil {
				t.Fatal(err)
			}
			checkChainOutgoing(t, toB, userAuthority, "A-token", aPath)
			proofB := authzTestSign(t, chainClaims(constant.SubjectTypeService, tc.user, "A", "B", bPath))
			atB := toB.Copy()
			atB.Delete(constant.ServiceAuthority)
			atB.Set(constant.AuthzSign, proofB)
			ctxB := chainEntry(t, atB, "B", bPath)
			checkChainRuntime(t, ctxB, "B", bPath, "A", aPath, initial.UserContext)

			// B 再调用 C：更换当前服务凭证和 Invoke 入口，独立保留原始用户。
			toC, err := authz.PrepareOutgoingAuthorityMetadata(ctxB, service.OutgoingTarget{
				TargetAppId: "C", TargetMethod: constant.RequestMethodGrpcString, TargetPath: cPath,
			}, chainAuthorityProvider("B-token"))
			if err != nil {
				t.Fatal(err)
			}
			checkChainOutgoing(t, toC, userAuthority, "B-token", bPath)
			proofC := authzTestSign(t, chainClaims(constant.SubjectTypeService, tc.user, "B", "C", cPath))
			atC := toC.Copy()
			atC.Delete(constant.ServiceAuthority)
			atC.Set(constant.AuthzSign, proofC)
			ctxC := chainEntry(t, atC, "C", cPath)
			checkChainRuntime(t, ctxC, "C", cPath, "B", bPath, initial.UserContext)
			if proofA == proofB || proofB == proofC {
				t.Fatal("authorization proof was reused across hops")
			}

			// 即使证明仍在有效期内，B 的旧允许结果也不能投递到 C。
			verification := authzTestOptions()
			verification.ExpectedServiceAppId = "C"
			verification.ExpectedServicePath = cPath
			if _, err := service.VerifyAuthzSign(proofB, verification); !errors.Is(err, service.ErrAuthzSignInvalidClaims) {
				t.Fatalf("B proof accepted at C: %v", err)
			}

			// 同一 B 请求顺序调用不同目标时，每份出站视图与 metadata 均独立持有。
			first, err := service.BuildOutgoingContext(ctxB, service.OutgoingTarget{
				TargetAppId: "C", TargetMethod: constant.RequestMethodGrpcString, TargetPath: cPath,
			}, "B-token")
			if err != nil {
				t.Fatal(err)
			}
			second, err := service.BuildOutgoingContext(ctxB, service.OutgoingTarget{
				TargetAppId: "D", TargetMethod: constant.RequestMethodGrpcString, TargetPath: "/d.Service/Other",
			}, "B-token")
			if err != nil {
				t.Fatal(err)
			}
			first.Metadata().Set(constant.InvokePath, "/mutated")
			if first.TargetAppId != "C" || first.TargetPath != cPath || second.TargetAppId != "D" || second.TargetPath != "/d.Service/Other" {
				t.Fatal("sequential calls shared target state")
			}
			checkChainOutgoing(t, second.Metadata(), userAuthority, "B-token", bPath)
			checkChainRuntime(t, ctxB, "B", bPath, "A", aPath, initial.UserContext)
			if atB.Get(constant.AuthzSign)[0] != proofB || toB.Get(constant.ServiceAuthority)[0] != "A-token" || incoming.Get(constant.AuthzSign)[0] != proofA {
				t.Fatal("outgoing calls mutated previous-hop inputs")
			}
		})
	}
}

// checkChainOutgoing 检查精确出站身份来源及旧证明不继承，不据明文头建立身份。
func checkChainOutgoing(t *testing.T, md metadata.MD, user, authority, invokePath string) {
	t.Helper()
	if service.ParseMetaKey(md, constant.UserAuthority) != user || service.ParseMetaKey(md, constant.ServiceAuthority) != authority ||
		service.ParseMetaKey(md, constant.InvokeMethod) != constant.RequestMethodGrpcString || service.ParseMetaKey(md, constant.InvokePath) != invokePath {
		t.Fatal("wrong outgoing credential or Invoke entry")
	}
	for _, key := range []string{constant.AuthzSign, constant.UserId, constant.Session, constant.AppId, constant.SubjectType, constant.DecisionId} {
		if len(md.Get(key)) != 0 {
			t.Fatal("unexpected inherited header: " + key)
		}
	}
}

// checkChainRuntime 检查本地服务事实、已验签上一跳身份以及完整用户视图。
func checkChainRuntime(t *testing.T, ctx context.Context, app, path, invokeApp, invokePath string, user *service.UserContext) {
	t.Helper()
	c := service.MustFromContext(ctx)
	if c.ServiceAppId != app || c.ServiceInstanceId != app+"-instance" || c.ServiceMethod != constant.RequestMethodGrpcString || c.ServicePath != path ||
		c.InvokeAppId != invokeApp || c.InvokeMethod != constant.RequestMethodGrpcString || c.InvokePath != invokePath ||
		c.AuthzSignContext.SubjectType != constant.SubjectTypeService || c.AuthzSignContext.SubjectAppId != invokeApp {
		t.Fatal("wrong local service, previous hop, or authorization subject")
	}
	if (c.UserContext == nil) != (user == nil) || (user != nil && *c.UserContext != *user) {
		t.Fatal("original user context was dropped or overwritten")
	}
	public, _ := metadata.FromIncomingContext(ctx)
	for _, key := range []string{constant.UserAuthority, constant.ServiceAuthority, constant.AuthzSign} {
		if len(public.Get(key)) != 0 {
			t.Fatal("private credential exposed in business metadata: " + key)
		}
	}
}
