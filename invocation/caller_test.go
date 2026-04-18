package invocation

import (
	"context"
	"testing"

	"github.com/fireflycore/go-micro/constant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestRemoteServiceCaller_Invoke_UsesBuildContextByDefault(t *testing.T) {
	invoked := false
	caller := NewRemoteServiceCaller(
		&UnaryInvoker{
			Dialer: testDialer{conn: &grpc.ClientConn{}},
			InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
				invoked = true

				md, ok := metadata.FromOutgoingContext(ctx)
				if !ok {
					t.Fatal("expected outgoing metadata")
				}
				if got := md.Get("x-firefly-user-id"); len(got) == 0 || got[0] != "u-100" {
					t.Fatalf("unexpected user id metadata: %v", got)
				}
				return nil
			},
		},
		&ServiceDNS{
			Service:   "auth",
			Namespace: "default",
		},
		func(ctx context.Context) *InvocationContext {
			return &InvocationContext{
				Caller: Caller{UserId: "u-100"},
			}
		},
	)

	err := caller.Invoke(context.Background(), "/acme.auth.app.v1.AuthAppService/GetAppSecret", &struct{}{}, &struct{}{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !invoked {
		t.Fatal("expected invoke func to be called")
	}
}

func TestRemoteServiceCaller_Invoke_ExplicitInvocationContextOverridesDefault(t *testing.T) {
	caller := NewRemoteServiceCaller(
		&UnaryInvoker{
			Dialer: testDialer{conn: &grpc.ClientConn{}},
			InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
				md, ok := metadata.FromOutgoingContext(ctx)
				if !ok {
					t.Fatal("expected outgoing metadata")
				}
				if got := md.Get("x-firefly-user-id"); len(got) == 0 || got[0] != "u-explicit" {
					t.Fatalf("unexpected user id metadata: %v", got)
				}
				return nil
			},
		},
		&ServiceDNS{
			Service:   "auth",
			Namespace: "default",
		},
		func(ctx context.Context) *InvocationContext {
			return &InvocationContext{
				Caller: Caller{UserId: "u-default"},
			}
		},
	)

	err := caller.Invoke(
		context.Background(),
		"/acme.auth.user.v1.AuthUserService/GetUser",
		&struct{}{},
		&struct{}{},
		WithInvocationContext(&InvocationContext{
			Caller: Caller{UserId: "u-explicit"},
		}),
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestRemoteServiceCaller_Invoke_ReturnsInvokerErrorWhenInvokerMissing(t *testing.T) {
	caller := NewRemoteServiceCaller(nil, &ServiceDNS{
		Service:   "auth",
		Namespace: "default",
	}, nil)

	err := caller.Invoke(context.Background(), "/acme.auth.app.v1.AuthAppService/GetAppSecret", &struct{}{}, &struct{}{})
	if err != ErrInvokerDialerIsNil {
		t.Fatalf("expected %v, got %v", ErrInvokerDialerIsNil, err)
	}
}

func TestBuildInvocationContextFromContext_PreservesMetadataAndUserContext(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", "req-1"))
	ctx = WithUserContext(ctx, &UserContextMeta{
		UserId:   "u-1",
		AppId:    "app-1",
		TenantId: "tenant-1",
		RoleIds:  []string{"r1", "r2"},
		OrgIds:   []string{"o1"},
	})

	out := BuildInvocationContextFromContext(ctx)
	if out == nil {
		t.Fatal("expected invocation context, got nil")
	}
	if got := out.Metadata.Get("x-request-id"); len(got) == 0 || got[0] != "req-1" {
		t.Fatalf("unexpected request metadata: %v", got)
	}
	if out.Caller.UserId != "u-1" || out.Caller.AppId != "app-1" || out.Caller.TenantId != "tenant-1" {
		t.Fatalf("unexpected caller: %+v", out.Caller)
	}
	if len(out.Caller.RoleIds) != 2 || len(out.Caller.OrgIds) != 1 {
		t.Fatalf("unexpected caller scopes: %+v", out.Caller)
	}
}

func TestBuildInvocationContextFromContext_UsesOutgoingMetadata(t *testing.T) {
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(constant.TraceId, "trace-1"))

	out := BuildInvocationContextFromContext(ctx)
	if out == nil {
		t.Fatal("expected invocation context, got nil")
	}
	if got := out.Metadata.Get(constant.TraceId); len(got) == 0 || got[0] != "trace-1" {
		t.Fatalf("unexpected trace metadata: %v", got)
	}
}
