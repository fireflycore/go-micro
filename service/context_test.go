package service

import (
	"context"
	"testing"

	"github.com/fireflycore/go-micro/constant"
	"go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/metadata"
)

func TestBuildContext(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.UserId, "user-1",
		constant.AppId, "app-1",
		constant.TenantId, "tenant-1",
		constant.SubjectType, constant.SubjectTypeUser,
		constant.InvokeAppId, "app-1",
		constant.TargetAppId, "order-app",
		constant.ResourceType, constant.RequestMethodGrpcString,
		constant.ResourcePath, "/acme.order.v1.OrderService/List",
		constant.DecisionId, "decision-1",
		constant.AuthzContext, "signed-context",
		constant.OrgIds, "org-1",
		constant.OrgIds, "org-2",
		constant.RoleIds, "role-1",
	))

	value := BuildContext(ctx, BuildContextOptions{
		ServiceAppId:      "svc-app",
		ServiceInstanceId: "svc-1",
	})
	if value == nil {
		t.Fatal("expected service context, got nil")
	}
	if value.UserId != "user-1" || value.AppId != "app-1" || value.TenantId != "tenant-1" {
		t.Fatalf("unexpected identity fields: %+v", value)
	}
	if value.SubjectType != constant.SubjectTypeUser || value.InvokeAppId != "app-1" || value.TargetAppId != "order-app" {
		t.Fatalf("unexpected authz identity fields: %+v", value)
	}
	if value.ResourceType != constant.RequestMethodGrpcString || value.ResourcePath != "/acme.order.v1.OrderService/List" || value.DecisionId != "decision-1" {
		t.Fatalf("unexpected authz resource fields: %+v", value)
	}
	if value.AuthzContextToken != "signed-context" {
		t.Fatalf("unexpected authz context token: %+v", value)
	}
	if value.ServiceAppId != "svc-app" || value.ServiceInstanceId != "svc-1" {
		t.Fatalf("unexpected service identity fields: %+v", value)
	}
	if len(value.OrgIds) != 2 || len(value.RoleIds) != 1 {
		t.Fatalf("unexpected scope fields: %+v", value)
	}
}

func TestBuildContext_UsesAppIdAsInvokeFallback(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.AppId, "legacy-app",
	))

	value := BuildContext(ctx, BuildContextOptions{})
	if value.AppId != "legacy-app" || value.InvokeAppId != "legacy-app" {
		t.Fatalf("expected app id fallback to populate invoke app id: %+v", value)
	}
}

func TestBuildContext_UsesTraceSpan(t *testing.T) {
	provider := trace.NewTracerProvider()
	defer func() { _ = provider.Shutdown(context.Background()) }()

	tracer := provider.Tracer("service-test")
	ctx, span := tracer.Start(context.Background(), "build-context")
	defer span.End()

	value := BuildContext(ctx, BuildContextOptions{})
	if value.TraceId == "" {
		t.Fatal("expected trace id to be populated from active span")
	}
}

func TestWithContextAndFromContext(t *testing.T) {
	base := context.Background()
	withValue := WithContext(base, &Context{UserId: "user-1"})

	value, ok := FromContext(withValue)
	if !ok {
		t.Fatal("expected service context in context")
	}
	if value.UserId != "user-1" {
		t.Fatalf("unexpected user id: %+v", value)
	}
}
