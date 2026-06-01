package invocation

import (
	"context"
	"testing"

	"github.com/fireflycore/go-micro/constant"
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
				if got := md.Get(constant.UserAuthority); len(got) == 0 || got[0] != "user-token" {
					t.Fatalf("unexpected user authority metadata: %v", got)
				}
				if got := md.Get(constant.UserId); len(got) != 0 {
					t.Fatalf("expected stale user id metadata to be removed: %v", got)
				}
				if got := md.Get("x-request-id"); len(got) != 0 {
					t.Fatalf("expected non-allowlisted request id to be removed: %v", got)
				}
				return nil
			},
		},
		&DNS{
			Service:   "auth",
			Namespace: "default",
		},
	)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.UserAuthority, "user-token",
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

func TestRemoteServiceCaller_Invoke_OverridesServiceAuthority(t *testing.T) {
	caller := NewRemoteServiceCaller(
		&UnaryInvoker{
			Dialer:                   testDialer{conn: &grpc.ClientConn{}},
			ServiceAuthorityProvider: fixedServiceAuthorityProvider("config-service-token"),
			InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
				md, ok := metadata.FromOutgoingContext(ctx)
				if !ok {
					t.Fatal("expected outgoing metadata")
				}
				if got := md.Get(constant.UserAuthority); len(got) == 0 || got[0] != "user-token" {
					t.Fatalf("unexpected user authority metadata: %v", got)
				}
				if got := md.Get(constant.UserId); len(got) != 0 {
					t.Fatalf("expected stale user id metadata to be removed: %v", got)
				}
				if got := md.Get(constant.ServiceAuthority); len(got) == 0 || got[0] != "config-service-token" {
					t.Fatalf("unexpected service authority metadata: %v", got)
				}
				return nil
			},
		},
		&DNS{
			Service:   "auth",
			Namespace: "default",
		},
	)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.UserAuthority, "user-token",
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
	caller := NewRemoteServiceCaller(nil, &DNS{
		Service:   "auth",
		Namespace: "default",
	})

	err := caller.Invoke(context.Background(), "/acme.auth.app.v1.AuthAppService/GetAppSecret", &struct{}{}, &struct{}{})
	if err != ErrInvokerDialerIsNil {
		t.Fatalf("expected %v, got %v", ErrInvokerDialerIsNil, err)
	}
}
