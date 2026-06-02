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
		constant.AppLanguage, "zh-CN",
		constant.Session, "session-1",
		constant.UserId, "user-1",
		constant.AppId, "app-1",
		constant.TenantId, "tenant-1",
		constant.SubjectType, constant.SubjectTypeUser,
		constant.InvokeAppId, "app-1",
		constant.InvokeInstanceId, "app-1-inst",
		constant.TargetAppId, "order-app",
		constant.TargetInstanceId, "order-app-inst",
		constant.ApiMethod, constant.RequestMethodGrpcString,
		constant.ApiPath, "/acme.order.v1.OrderService/List",
		constant.DecisionId, "decision-1",
		constant.AuthzSign, "signed-context",
		constant.OrgIds, "org-1",
		constant.OrgIds, "org-2",
		constant.PostIds, "post-1",
		constant.RoleIds, "role-1",
	))

	value := BuildContext(ctx, BuildContextOptions{})
	if value == nil {
		t.Fatal("expected service context, got nil")
	}
	if value.UserId != "user-1" || value.AppId != "app-1" || value.TenantId != "tenant-1" {
		t.Fatalf("unexpected identity fields: %+v", value)
	}
	if value.AppLanguage != "zh-CN" || value.Session != "session-1" {
		t.Fatalf("unexpected app context fields: %+v", value)
	}
	if value.SubjectType != constant.SubjectTypeUser || value.InvokeAppId != "app-1" || value.TargetAppId != "order-app" {
		t.Fatalf("unexpected authz identity fields: %+v", value)
	}
	if value.InvokeInstanceId != "app-1-inst" || value.TargetInstanceId != "order-app-inst" {
		t.Fatalf("unexpected service instance fields: %+v", value)
	}
	if value.ApiMethod != constant.RequestMethodGrpcString || value.ApiPath != "/acme.order.v1.OrderService/List" {
		t.Fatalf("unexpected api fields: %+v", value)
	}
	if value.DecisionId != "decision-1" {
		t.Fatalf("unexpected decision id: %+v", value)
	}
	if value.AuthzSignJWS != "signed-context" {
		t.Fatalf("unexpected authz sign token: %+v", value)
	}
	if value.UserContext == nil || value.UserContext.AppId != "app-1" || value.UserContext.Session != "session-1" {
		t.Fatalf("unexpected grouped user context: %+v", value)
	}
	if value.DecisionContext == nil || value.DecisionContext.TargetAppId != "order-app" {
		t.Fatalf("unexpected grouped decision context: %+v", value)
	}
	if value.DecisionContext.InvokeAppId != "app-1" || value.DecisionContext.InvokeInstanceId != "app-1-inst" {
		t.Fatalf("unexpected grouped decision invoke fields: %+v", value.DecisionContext)
	}
	if value.InvokeServiceContext != nil {
		t.Fatalf("expected user subject not to derive invoke service context: %+v", value.InvokeServiceContext)
	}
	if value.TargetServiceContext == nil || value.TargetServiceContext.InstanceId != "order-app-inst" {
		t.Fatalf("unexpected target service context: %+v", value)
	}
	if len(value.OrgIds) != 2 || len(value.PostIds) != 1 || len(value.RoleIds) != 1 {
		t.Fatalf("unexpected scope fields: %+v", value)
	}
}

func TestBuildContext_DoesNotUseAppIdAsInvokeFallback(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.AppId, "user-app",
	))

	value := BuildContext(ctx, BuildContextOptions{})
	if value.AppId != "user-app" || value.InvokeAppId != "" {
		t.Fatalf("expected user app id to stay separate from invoke app id: %+v", value)
	}
}

func TestBuildContext_BuildsInvokeServiceContextForServiceSubject(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.SubjectType, constant.SubjectTypeService,
		constant.InvokeAppId, "service-a",
		constant.InvokeInstanceId, "service-a-1",
		constant.TargetAppId, "service-b",
	))

	value := BuildContext(ctx, BuildContextOptions{})
	if value.InvokeServiceContext == nil || value.InvokeServiceContext.AppId != "service-a" || value.InvokeServiceContext.InstanceId != "service-a-1" {
		t.Fatalf("expected service subject to build invoke service context: %+v", value)
	}
	if value.UserContext != nil {
		t.Fatalf("expected service subject without user fields to keep user context empty: %+v", value.UserContext)
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
