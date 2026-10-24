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
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
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
