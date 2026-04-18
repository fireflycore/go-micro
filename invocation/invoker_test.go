package invocation

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type testDialer struct {
	conn *grpc.ClientConn
	err  error
}

func (d testDialer) Dial(ctx context.Context, service *ServiceDNS) (*grpc.ClientConn, error) {
	return d.conn, d.err
}

func (d testDialer) Close() error {
	return nil
}

type testAuthorizer struct {
	called bool
	err    error
}

func (a *testAuthorizer) Authorize(ctx context.Context, input *AuthzContext) error {
	a.called = true
	return a.err
}

func TestUnaryInvoker_Invoke_CallsAuthorizerAndInvokeFunc(t *testing.T) {
	authorizer := &testAuthorizer{}
	invoked := false

	invoker := &UnaryInvoker{
		Dialer:     testDialer{conn: &grpc.ClientConn{}},
		Authorizer: authorizer,
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			invoked = true

			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				t.Fatalf("expected outgoing metadata")
			}
			if md.Get("x-firefly-user-id")[0] != "u-1" {
				t.Fatalf("unexpected user id metadata: %v", md.Get("x-firefly-user-id"))
			}

			return nil
		},
	}

	err := invoker.Invoke(context.Background(), &ServiceDNS{
		Service:   "auth",
		Namespace: "default",
	}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{},
		WithInvocationContext(&InvocationContext{
			Caller: Caller{UserId: "u-1"},
		}),
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !authorizer.called {
		t.Fatalf("expected authorizer to be called")
	}
	if !invoked {
		t.Fatalf("expected invoke func to be called")
	}
}

func TestUnaryInvoker_Invoke_ReturnsAuthorizerError(t *testing.T) {
	expectedErr := errors.New("deny")
	invoker := &UnaryInvoker{
		Dialer: testDialer{conn: &grpc.ClientConn{}},
		Authorizer: &testAuthorizer{
			err: expectedErr,
		},
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			t.Fatalf("invoke func should not be called when authz fails")
			return nil
		},
	}

	err := invoker.Invoke(context.Background(), &ServiceDNS{
		Service:   "auth",
		Namespace: "default",
	}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestUnaryInvoker_Invoke_WithoutInvocationContextDoesNotPanic(t *testing.T) {
	invoked := false
	invoker := &UnaryInvoker{
		Dialer: testDialer{conn: &grpc.ClientConn{}},
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			invoked = true
			return nil
		},
	}

	err := invoker.Invoke(context.Background(), &ServiceDNS{
		Service:   "auth",
		Namespace: "default",
	}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !invoked {
		t.Fatal("expected invoke func to be called")
	}
}
