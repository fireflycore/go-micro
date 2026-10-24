package gm

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// TestActualEntryBinding 验证以适配器实际方法校验投递绑定，非业务入口跳过验签但不建立可信身份。
func TestActualEntryBinding(t *testing.T) {
	v := contextTestOptions()
	i := NewServiceContextUnaryInterceptor(ServiceContextInterceptorOptions{ServiceAppId: "B", ServiceInstanceId: "local", AuthzVerification: &v, AuthzSkipMethods: []string{"/health"}})
	for _, path := range []string{"/items.Service/Get", "/other", "/health"} {
		md := metadata.Pairs(constant.AuthzSign, signContextTestClaims(t, contextTestClaims()), constant.XRealIp, "forged")
		ctx := metadata.NewIncomingContext(context.Background(), md)
		ctx = peer.NewContext(ctx, &peer.Peer{Addr: &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 1}})
		called := false
		_, err := i(ctx, nil, &grpc.UnaryServerInfo{FullMethod: path}, func(ctx context.Context, _ any) (any, error) {
			called = true
			c := service.MustFromContext(ctx)
			if c.ServicePath != path || c.ClientIp != "192.0.2.1" {
				t.Fatal(c)
			}
			if (c.AuthzSignContext == nil) != (path == "/health") {
				t.Fatal("skip created verified identity")
			}
			md, _ := metadata.FromIncomingContext(ctx)
			if len(md.Get(constant.AuthzSign)) != 0 {
				t.Fatal("proof exposed")
			}
			return nil, nil
		})
		if path == "/other" {
			if err == nil || called {
				t.Fatal("accepted cross-method proof")
			}
		} else if err != nil || !called {
			t.Fatal(err)
		}
	}
}

// testServerStream 只替换测试流的上下文，无需建立网络连接。
type testServerStream struct {
	grpc.ServerStream                 // 保留流接口；本测试不执行消息收发。
	ctx               context.Context // 模拟原始入站上下文及其取消信号。
}

func (s *testServerStream) Context() context.Context { return s.ctx }

// TestStreamingContextBindingAndCancellation 验证流入口绑定、受控 IP 提取、凭证隐藏和取消传递。
func TestStreamingContextBindingAndCancellation(t *testing.T) {
	v := contextTestOptions()
	interceptor := NewServiceContextStreamInterceptor(ServiceContextInterceptorOptions{
		ServiceAppId: "B", AuthzVerification: &v,
		TrustedClientIP: func(context.Context, metadata.MD) string { return "trusted" },
	})
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	md := metadata.Pairs(constant.AuthzSign, signContextTestClaims(t, contextTestClaims()), constant.UserAuthority, "secret")
	stream := &testServerStream{ctx: metadata.NewIncomingContext(parent, md)}
	called := false
	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/items.Service/Get", IsClientStream: true, IsServerStream: true}, func(_ any, s grpc.ServerStream) error {
		called = true
		c := service.MustFromContext(s.Context())
		if c.ServicePath != "/items.Service/Get" || c.InvokeAppId != "A" || c.ClientIp != "trusted" {
			t.Fatal("stream context not bound")
		}
		public, _ := metadata.FromIncomingContext(s.Context())
		if len(public.Get(constant.UserAuthority)) != 0 {
			t.Fatal("stream exposed credential")
		}
		cancel()
		if s.Context().Err() != context.Canceled {
			t.Fatal("stream lost cancellation")
		}
		return nil
	})
	if err != nil || !called {
		t.Fatal(err)
	}
	err = interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/wrong"}, func(any, grpc.ServerStream) error {
		t.Fatal("accepted wrong stream binding")
		return nil
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatal(err)
	}
}

// contextTestNow 固定本包拦截器与真实 gRPC 回环测试的时钟。
var contextTestNow = time.Unix(1792627200, 0)

// contextTestKey 是本包专用的全零种子测试密钥，不能用于实际服务。
var contextTestKey = ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))

// contextTestClaims 只保留本包使用的服务主体加用户样本，独立表达 HTTP 授权与 gRPC 投递。
func contextTestClaims() map[string]any {
	return map[string]any{
		"iss": "firefly-authz", "sub": "A", "subject_type": "service", "subject_app_id": "A", "invoke_service_app_id": "A",
		"method": "POST", "path": "/v1/items", "service_app_id": "B", "service_method": "GRPC", "service_path": "/items.Service/Get",
		"decision": "allow", "decision_id": "decision", "iat": contextTestNow.Unix(), "exp": contextTestNow.Unix() + 60,
		"user_context": map[string]any{"session": "secret-session", "user_id": "user", "app_id": "login-app", "tenant_id": "user-tenant"},
	}
}

// signContextTestClaims 在测试实际投递路径确定后签名，支持错绑定负例。
func signContextTestClaims(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	input := enc([]byte(`{"alg":"EdDSA","kid":"default","typ":"JWT"}`)) + "." + enc(payload)
	return input + "." + enc(ed25519.Sign(contextTestKey, []byte(input)))
}

// contextTestOptions 的预期绑定来自本地测试入口，不读取待验证的 claims。
func contextTestOptions() service.AuthzSignVerificationOptions {
	return service.AuthzSignVerificationOptions{
		PublicKeys: map[string]ed25519.PublicKey{"default": contextTestKey.Public().(ed25519.PublicKey)}, Issuer: "firefly-authz",
		ExpectedServiceAppId: "B", ExpectedServiceMethod: "GRPC", ExpectedServicePath: "/items.Service/Get",
		Now: func() time.Time { return contextTestNow },
	}
}

// TestMissingVerificationFailsClosed 验证 unary 与 stream 都不把漏装配置当作全局豁免。
func TestMissingVerificationFailsClosed(t *testing.T) {
	options := ServiceContextInterceptorOptions{ServiceAppId: "B", AuthzSkipMethods: []string{"/health"}}
	unary := NewServiceContextUnaryInterceptor(options)
	stream := NewServiceContextStreamInterceptor(options)
	for _, path := range []string{"/items.Service/Get", "/health/extra", "/health"} {
		t.Run(path, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(constant.UserId, "forged"))
			calls := 0
			check := func(ctx context.Context) {
				calls++
				c := service.MustFromContext(ctx)
				if c.UserContext != nil || c.AuthzSignContext != nil || c.InvokeAppId != "" {
					t.Fatal("independent entry created trusted identity")
				}
			}
			_, unaryErr := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: path}, func(ctx context.Context, _ any) (any, error) {
				check(ctx)
				return nil, nil
			})
			streamErr := stream(nil, &testServerStream{ctx: ctx}, &grpc.StreamServerInfo{FullMethod: path}, func(_ any, s grpc.ServerStream) error {
				check(s.Context())
				return nil
			})
			if path == "/health" {
				if unaryErr != nil || streamErr != nil || calls != 2 {
					t.Fatalf("independent entry failed: unary=%v stream=%v calls=%d", unaryErr, streamErr, calls)
				}
			} else if status.Code(unaryErr) != codes.Internal || status.Code(streamErr) != codes.Internal || calls != 0 {
				t.Fatalf("missing verification accepted: unary=%v stream=%v calls=%d", unaryErr, streamErr, calls)
			}
		})
	}
}
