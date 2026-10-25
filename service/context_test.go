package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/service"
	"github.com/fireflycore/go-micro/werror"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// entry 固定本地服务事实，便于区分明文输入和验签后身份。
func entry(t *testing.T, md metadata.MD, verify bool) context.Context {
	t.Helper()
	o := service.BuildContextOptions{ServiceAppId: "B", ServiceInstanceId: "local", ServiceMethod: "GRPC", ServicePath: "/items.Service/Get"}
	if verify {
		v := authzTestOptions()
		o.AuthzVerification = &v
	}
	ctx, err := service.NewIncomingContext(context.Background(), md, o)
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

// 不验签的独立入口不能从伪造用户、Invoke 或本地服务头建立可信身份。
func TestIncomingDoesNotTrustObservation(t *testing.T) {
	md := metadata.Pairs(constant.UserId, "forged", constant.Session, "secret", constant.InvokeMethod, "GET", constant.InvokePath, "/forged",
		"x-firefly-service-app-id", "fake", constant.ServiceAuthority, "service-secret", constant.UserAuthority, "user-secret", constant.AcceptLanguage, "zh-CN, en;q=0.5")
	ctx := entry(t, md, false)
	c := service.MustFromContext(ctx)
	if c.UserContext != nil || c.AuthzSignContext != nil || c.InvokeAppId != "" || c.InvokePath != "" ||
		c.ServiceAppId != "B" || c.AppLanguage != "zh-CN" {
		t.Fatalf("untrusted projection: %+v", c)
	}
	public, _ := metadata.FromIncomingContext(ctx)
	for _, k := range []string{constant.UserAuthority, constant.ServiceAuthority, constant.AuthzSign, "x-firefly-service-app-id"} {
		if len(public.Get(k)) != 0 {
			t.Fatal(k)
		}
	}
	encoded, _ := json.Marshal(c)
	if strings.Contains(string(encoded), "secret") {
		t.Fatal("credential in business context")
	}
}

// 必需用户读取只检查已验证用户视图，不把服务主体误判为必须没有用户。
func TestRequireUserFromContextIdentityBranches(t *testing.T) {
	for _, tc := range []struct {
		// name 区分四种合法身份组合。
		name string
		// subject 是已验证的本跳授权主体，不等同于用户是否存在。
		subject string
		// user 表示签名是否保留独立用户视图。
		user bool
	}{
		{name: "anonymous", subject: "anonymous"},
		{name: "user", subject: "user", user: true},
		{name: "service", subject: "service"},
		{name: "dual", subject: "service", user: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := entry(t, metadata.Pairs(constant.AuthzSign, authzTestSign(t, authzTestClaims(tc.subject, tc.user))), true)
			user, err := service.RequireUserFromContext(ctx)
			if !tc.user {
				if user != nil || status.Code(err) != codes.Unauthenticated || werror.CodeOf(err) != codes.Unauthenticated {
					t.Fatalf("expected user requirement error: user=%v err=%v", user, err)
				}
				return
			}
			want := service.UserContext{Session: "secret-session", UserId: "user", AppId: "login-app", TenantId: "user-tenant"}
			if err != nil || user == nil || *user != want {
				t.Fatalf("verified user changed: user=%v err=%v", user, err)
			}
		})
	}
}

// 修改返回的用户不能污染请求级视图或后续读取结果。
func TestRequireUserFromContextReturnsCopy(t *testing.T) {
	ctx := entry(t, metadata.Pairs(constant.AuthzSign, authzTestSign(t, authzTestClaims("service", true))), true)
	user, err := service.RequireUserFromContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := *user
	*user = service.UserContext{UserId: "changed", AppId: "changed", TenantId: "changed", Session: "changed"}
	next, err := service.RequireUserFromContext(ctx)
	if err != nil || next == nil || *next != want || *service.MustFromContext(ctx).UserContext != want {
		t.Fatal("user accessor mutated stored context")
	}
}

// 普通身份头和原始凭证不能由辅助函数提升为可信用户，错误也不得泄露其内容。
func TestRequireUserFromContextRejectsUnverifiedInput(t *testing.T) {
	ctx := entry(t, metadata.Pairs(constant.UserId, "forged-user", constant.UserAuthority, "secret-token", constant.Session, "secret-session"), false)
	user, err := service.RequireUserFromContext(ctx)
	if user != nil || status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unverified input became user identity: %v", err)
	}
	if err.Error() != "user context is required" {
		t.Fatal("user requirement error exposed input or changed its stable message")
	}
}

// 缺少运行时上下文是装配错误，不能被伪装成合法匿名请求或自动创建后台上下文。
func TestRequireUserFromContextPreservesMissingContextFailure(t *testing.T) {
	for _, tc := range []struct {
		// name 标记调用方漏传上下文和入口未装配两种程序错误。
		name string
		// ctx 刻意不携带运行时视图，验证沿用 MustFromContext 契约。
		ctx context.Context
	}{
		{name: "nil"},
		{name: "missing-service-context", ctx: context.Background()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if got := recover(); got != "service: context not found in context" {
					t.Fatalf("missing context failure changed: %v", got)
				}
			}()
			_, _ = service.RequireUserFromContext(tc.ctx)
		})
	}
}

// 验证双身份保留、读取副本隔离、并发出站白名单和当次活动 span。
func TestVerifiedProjectionAndOutgoingIsolation(t *testing.T) {
	md := metadata.Pairs(constant.AuthzSign, authzTestSign(t, authzTestClaims("service", true)),
		constant.UserAuthority, "user-secret", constant.InvokeMethod, "POST", constant.InvokePath, "/a", constant.AppVersion, "1",
		"x-firefly-unknown", "forged", constant.TraceParent, "forged-trace", constant.XRealIp, "forged-ip")
	ctx := entry(t, md, true)
	c := service.MustFromContext(ctx)
	if c.InvokeAppId != "A" || c.UserContext.AppId != "login-app" || c.AuthzSignContext.SubjectAppId != "A" {
		t.Fatal("lost dual identity")
	}
	original := c.UserContext.UserId
	// 修改读取副本不能污染存储视图，后续出站也不读取这个可变副本。
	c.UserContext.UserId = "changed"
	if service.MustFromContext(ctx).UserContext.UserId != original {
		t.Fatal("shared runtime pointer")
	}
	if strings.Contains(fmt.Sprintf("%+v", c.UserContext), "secret-session") {
		t.Fatal("session in default formatting")
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: trace.TraceID{1}, SpanID: trace.SpanID{2}, TraceFlags: trace.FlagsSampled})
	ctx = trace.ContextWithSpanContext(ctx, sc)
	var wg sync.WaitGroup
	// 同一入口并发访问不同目标，目标和 metadata 必须由各次调用独占。
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := fmt.Sprintf("/C/%d", i)
			out, err := service.BuildOutgoingContext(ctx, service.OutgoingTarget{TargetAppId: "C", TargetMethod: "GRPC", TargetPath: path}, "B-token")
			if err != nil {
				t.Error(err)
				return
			}
			outMD := out.Metadata()
			if out.TargetPath != path || out.InvokePath != "/items.Service/Get" || outMD.Get(constant.UserAuthority)[0] != "user-secret" ||
				outMD.Get(constant.ServiceAuthority)[0] != "B-token" || !strings.Contains(out.TraceParent, sc.SpanID().String()) {
				t.Error("wrong outgoing")
			}
			for _, k := range []string{constant.AuthzSign, constant.XRealIp, constant.UserId, constant.AcceptLanguage, "x-firefly-unknown"} {
				if len(outMD.Get(k)) != 0 {
					t.Error("propagated " + k)
				}
			}
			outMD.Set(constant.AppVersion, "mutated")
		}(i)
	}
	wg.Wait()
	if service.MustFromContext(ctx).ServicePath != "/items.Service/Get" || md.Get(constant.AppVersion)[0] != "1" {
		t.Fatal("outbound modified input")
	}
}

// 重复标量、控制字符和越界长度直接拒绝，不采用静默截断或取首值。
func TestIncomingScalarLimits(t *testing.T) {
	for _, v := range []metadata.MD{
		metadata.Pairs(constant.InvokeMethod, "GET", constant.InvokeMethod, "POST"),
		metadata.Pairs(constant.InvokePath, "/bad\npath"),
		metadata.Pairs(constant.InvokeMethod, strings.Repeat("x", 33)),
		metadata.Pairs(constant.InvokePath, strings.Repeat("x", 2049)),
	} {
		if _, err := service.CollectIncomingContext(v, "GRPC", "/entry", ""); err == nil {
			t.Fatal("accepted invalid input")
		}
	}
}

// TestOutgoingRequiresExplicitContext 验证出站构建不将 nil 伪装成无请求身份的后台调用。
func TestOutgoingRequiresExplicitContext(t *testing.T) {
	target := service.OutgoingTarget{TargetMethod: "GRPC", TargetPath: "/B/Get"}
	if _, err := service.BuildOutgoingContext(context.Background(), target, "A-token"); err != nil {
		t.Fatalf("explicit background context failed: %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("nil context was silently accepted")
		}
	}()
	_, _ = service.BuildOutgoingContext(nil, target, "A-token")
}
