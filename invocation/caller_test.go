package invocation

import (
	"context"
	"testing"

	"github.com/fireflycore/go-micro/constant"
	svc "github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestRemoteServiceCaller_Invoke_ReusesIncomingMetadataByDefault(t *testing.T) {
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
				if got := md.Get("x-request-id"); len(got) == 0 || got[0] != "req-1" {
					t.Fatalf("unexpected request metadata: %v", got)
				}
				return nil
			},
		},
		&svc.DNS{
			Service:   "auth",
			Namespace: "default",
		},
	)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.UserId, "u-100",
		"x-request-id", "req-1",
	))

	err := caller.Invoke(ctx, "/acme.auth.app.v1.AuthAppService/GetAppSecret", &struct{}{}, &struct{}{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !invoked {
		t.Fatal("expected invoke func to be called")
	}
}

func TestRemoteServiceCaller_Invoke_InjectsCallerServiceIdentity(t *testing.T) {
	caller := NewRemoteServiceCaller(
		&UnaryInvoker{
			Dialer:            testDialer{conn: &grpc.ClientConn{}},
			ServiceAppId:      "config",
			ServiceInstanceId: "config-1",
			InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
				md, ok := metadata.FromOutgoingContext(ctx)
				if !ok {
					t.Fatal("expected outgoing metadata")
				}
				if got := md.Get("x-firefly-user-id"); len(got) == 0 || got[0] != "u-incoming" {
					t.Fatalf("unexpected user id metadata: %v", got)
				}
				if got := md.Get(constant.ServiceAppId); len(got) == 0 || got[0] != "config" {
					t.Fatalf("unexpected service app id metadata: %v", got)
				}
				if got := md.Get(constant.ServiceInstanceId); len(got) == 0 || got[0] != "config-1" {
					t.Fatalf("unexpected service instance id metadata: %v", got)
				}
				return nil
			},
		},
		&svc.DNS{
			Service:   "auth",
			Namespace: "default",
		},
	)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.UserId, "u-incoming",
	))

	err := caller.Invoke(
		ctx,
		"/acme.auth.user.v1.AuthUserService/GetUser",
		&struct{}{},
		&struct{}{},
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestRemoteServiceCaller_Invoke_ReturnsInvokerErrorWhenInvokerMissing(t *testing.T) {
	caller := NewRemoteServiceCaller(nil, &svc.DNS{
		Service:   "auth",
		Namespace: "default",
	})

	err := caller.Invoke(context.Background(), "/acme.auth.app.v1.AuthAppService/GetAppSecret", &struct{}{}, &struct{}{})
	if err != ErrInvokerDialerIsNil {
		t.Fatalf("expected %v, got %v", ErrInvokerDialerIsNil, err)
	}
}
