package invocation

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestRemoteServiceCaller_Invoke_UsesBuildContextByDefault(t *testing.T) {
	invoked := false
	caller := &RemoteServiceCaller{
		Service: &ServiceDNS{
			Service:   "auth",
			Namespace: "default",
		},
		Invoker: &UnaryInvoker{
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
		BuildContext: func(ctx context.Context) *InvocationContext {
			return &InvocationContext{
				Caller: Caller{UserId: "u-100"},
			}
		},
	}

	err := caller.Invoke(context.Background(), "/acme.auth.app.v1.AuthAppService/GetAppSecret", &struct{}{}, &struct{}{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !invoked {
		t.Fatal("expected invoke func to be called")
	}
}

func TestRemoteServiceCaller_Invoke_ExplicitInvocationContextOverridesDefault(t *testing.T) {
	caller := &RemoteServiceCaller{
		Service: &ServiceDNS{
			Service:   "auth",
			Namespace: "default",
		},
		Invoker: &UnaryInvoker{
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
		BuildContext: func(ctx context.Context) *InvocationContext {
			return &InvocationContext{
				Caller: Caller{UserId: "u-default"},
			}
		},
	}

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
	caller := &RemoteServiceCaller{
		Service: &ServiceDNS{
			Service:   "auth",
			Namespace: "default",
		},
	}

	err := caller.Invoke(context.Background(), "/acme.auth.app.v1.AuthAppService/GetAppSecret", &struct{}{}, &struct{}{})
	if err != ErrInvokerDialerIsNil {
		t.Fatalf("expected %v, got %v", ErrInvokerDialerIsNil, err)
	}
}
