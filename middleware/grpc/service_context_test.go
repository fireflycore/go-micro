package gm

import (
	"context"
	"testing"

	"github.com/fireflycore/go-micro/constant"
	servicectx "github.com/fireflycore/go-micro/service"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestNewServiceContextUnaryInterceptor(t *testing.T) {
	original := otel.GetTracerProvider()
	provider := trace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	defer func() {
		otel.SetTracerProvider(original)
		_ = provider.Shutdown(context.Background())
	}()

	tracer := provider.Tracer("gm-test")
	baseCtx, span := tracer.Start(context.Background(), "interceptor")
	defer span.End()

	baseCtx = metadata.NewIncomingContext(baseCtx, metadata.Pairs(
		constant.UserId, "user-1",
		constant.AppId, "app-1",
		constant.TenantId, "tenant-1",
		constant.OrgIds, "org-1",
		constant.RoleIds, "role-1",
		constant.RouteMethod, constant.RouteMethodService,
	))

	interceptor := NewServiceContextUnaryInterceptor(ServiceContextInterceptorOptions{
		ServiceAppId:      "svc-app",
		ServiceInstanceId: "svc-1",
	})

	resp, err := interceptor(baseCtx, &struct{}{}, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		value, ok := servicectx.FromContext(ctx)
		if !ok {
			t.Fatal("expected service context in handler context")
		}
		if value.UserId != "user-1" || value.ServiceAppId != "svc-app" {
			t.Fatalf("unexpected service context: %+v", value)
		}
		if value.TraceId == "" {
			t.Fatal("expected trace id from active span")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}
}
