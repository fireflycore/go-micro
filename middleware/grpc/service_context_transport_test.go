package gm

import (
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/service"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// contextHealthServer 借用已有健康协议作为测试载体，不把本用例配置为跳过验签的健康入口。
type contextHealthServer struct {
	healthpb.UnimplementedHealthServer                                     // 复用未测试方法的默认实现，不新增测试专用 protobuf。
	check                              func(context.Context, string) error // 在真实服务端处理上下文中核对可信身份与实际入口。
}

func (s *contextHealthServer) Check(ctx context.Context, _ *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	if err := s.check(ctx, healthpb.Health_Check_FullMethodName); err != nil {
		return nil, err
	}
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (s *contextHealthServer) Watch(_ *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error {
	if err := s.check(stream.Context(), healthpb.Health_Watch_FullMethodName); err != nil {
		return err
	}
	return stream.Send(&healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING})
}

// TestGRPCTransportBindingAndPrivateMetadata 经过真实编解码与 unary/stream 传输，不依赖 Python 或外部端口。
func TestGRPCTransportBindingAndPrivateMetadata(t *testing.T) {
	verification := contextTestOptions()
	options := ServiceContextInterceptorOptions{
		ServiceAppId: "B", ServiceInstanceId: "b-local", AuthzVerification: &verification,
	}
	server := grpc.NewServer(
		grpc.UnaryInterceptor(NewServiceContextUnaryInterceptor(options)),
		grpc.StreamInterceptor(NewServiceContextStreamInterceptor(options)),
	)
	var calls atomic.Int32
	healthpb.RegisterHealthServer(server, &contextHealthServer{check: func(ctx context.Context, path string) error {
		calls.Add(1)
		c := service.MustFromContext(ctx)
		if c.ServiceAppId != "B" || c.ServiceInstanceId != "b-local" || c.ServiceMethod != "GRPC" || c.ServicePath != path ||
			c.InvokeAppId != "A" || c.UserContext == nil || c.UserContext.AppId != "login-app" ||
			c.AuthzSignContext.Method != "POST" || c.AuthzSignContext.Path != "/v1/items" {
			return status.Error(codes.Internal, "incorrect verified service context")
		}
		md, _ := metadata.FromIncomingContext(ctx)
		for _, key := range []string{constant.UserAuthority, constant.ServiceAuthority, constant.AuthzSign, "x-firefly-service-app-id", "x-firefly-unknown"} {
			if len(md.Get(key)) != 0 {
				return status.Error(codes.Internal, "private metadata reached business handler")
			}
		}
		if !trace.SpanContextFromContext(ctx).IsValid() {
			return status.Error(codes.Internal, "trace context was lost")
		}
		return nil
	}})
	listener := bufconn.Listen(1024 * 1024)
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
		if err := <-served; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("serve failed: %v", err)
		}
	})
	conn, err := grpc.NewClient("passthrough:///context-test",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := healthpb.NewHealthClient(conn)
	parent, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	requestContext := func(path string) context.Context {
		claims := contextTestClaims()
		// 授权 API 保持 HTTP；仅投递绑定使用本次 gRPC 方法，不能隐式回退。
		claims["service_path"] = path
		return metadata.NewOutgoingContext(parent, metadata.Pairs(
			constant.AuthzSign, signContextTestClaims(t, claims), constant.UserAuthority, "private-user-token",
			constant.ServiceAuthority, "must-be-consumed", "x-firefly-service-app-id", "forged",
			"x-firefly-unknown", "drop", constant.TraceParent, "00-1234567890abcdef1234567890abcdef-1234567890abcdef-01",
		))
	}
	if _, err := client.Check(requestContext(healthpb.Health_Check_FullMethodName), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	// 同一有效签名投递到其他方法必须拒绝，且不能进入业务 handler。
	if _, err := client.Check(requestContext("/wrong"), &healthpb.HealthCheckRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("cross-method proof accepted: %v", err)
	}
	if _, err := client.Check(parent, &healthpb.HealthCheckRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unsigned request accepted: %v", err)
	}
	stream, err := client.Watch(requestContext(healthpb.Health_Watch_FullMethodName), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response, err := stream.Recv(); err != nil || response.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("stream context failed: response=%v err=%v", response, err)
	}
	if _, err := stream.Recv(); !errors.Is(err, io.EOF) {
		t.Fatalf("stream did not finish: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("unexpected handler calls: %d", calls.Load())
	}
}
