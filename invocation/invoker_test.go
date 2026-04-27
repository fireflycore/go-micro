package invocation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fireflycore/go-micro/constant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type testDialer struct {
	conn *grpc.ClientConn
	err  error
}

func (d testDialer) Dial(ctx context.Context, service *DNS) (*grpc.ClientConn, error) {
	return d.conn, d.err
}

func (d testDialer) Close() error {
	return nil
}

func TestNewUnaryInvoker_SetsCallerServiceIdentity(t *testing.T) {
	invoker := NewUnaryInvoker(testDialer{conn: &grpc.ClientConn{}}, "config", "config-1", 0)
	if invoker == nil {
		t.Fatal("expected invoker")
	}
	if invoker.ServiceAppId != "config" || invoker.ServiceInstanceId != "config-1" {
		t.Fatalf("unexpected invoker identity: %+v", invoker)
	}
	if invoker.Timeout != DefaultInvokeTimeout {
		t.Fatalf("unexpected invoker timeout: %s", invoker.Timeout)
	}
}

func TestUnaryInvoker_Invoke_ReusesIncomingMetadataAndInvokeFunc(t *testing.T) {
	invoked := false

	invoker := &UnaryInvoker{
		Dialer: testDialer{conn: &grpc.ClientConn{}},
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

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.UserId, "u-1",
	))

	err := invoker.Invoke(ctx, &DNS{
		Service:   "auth",
		Namespace: "default",
	}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !invoked {
		t.Fatalf("expected invoke func to be called")
	}
}

func TestUnaryInvoker_Invoke_WithoutExtraOptionsDoesNotPanic(t *testing.T) {
	invoked := false
	invoker := &UnaryInvoker{
		Dialer: testDialer{conn: &grpc.ClientConn{}},
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			invoked = true
			return nil
		},
	}

	err := invoker.Invoke(context.Background(), &DNS{
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

func TestUnaryInvoker_Invoke_InjectsCallerServiceIdentity(t *testing.T) {
	invoker := &UnaryInvoker{
		Dialer:            testDialer{conn: &grpc.ClientConn{}},
		ServiceAppId:      "config",
		ServiceInstanceId: "config-1",
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				t.Fatal("expected outgoing metadata")
			}
			if got := md.Get(constant.UserId); len(got) == 0 || got[0] != "u-incoming" {
				t.Fatalf("unexpected inherited user id metadata: %v", got)
			}
			if got := md.Get(constant.ServiceAppId); len(got) == 0 || got[0] != "config" {
				t.Fatalf("unexpected service app id metadata: %v", got)
			}
			if got := md.Get(constant.ServiceInstanceId); len(got) == 0 || got[0] != "config-1" {
				t.Fatalf("unexpected service instance id metadata: %v", got)
			}
			return nil
		},
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.UserId, "u-incoming",
	))

	err := invoker.Invoke(ctx, &DNS{
		Service:   "auth",
		Namespace: "default",
	}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestUnaryInvoker_Invoke_UsesConfiguredTimeout(t *testing.T) {
	invoker := &UnaryInvoker{
		Dialer:  testDialer{conn: &grpc.ClientConn{}},
		Timeout: 200 * time.Millisecond,
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("expected deadline")
			}
			remaining := time.Until(deadline)
			if remaining <= 0 || remaining > time.Second {
				t.Fatalf("unexpected configured timeout remaining: %s", remaining)
			}
			return nil
		},
	}

	err := invoker.Invoke(context.Background(), &DNS{
		Service:   "auth",
		Namespace: "default",
	}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestUnaryInvoker_Invoke_UsesDefaultTimeoutWhenUnset(t *testing.T) {
	invoker := &UnaryInvoker{
		Dialer: testDialer{conn: &grpc.ClientConn{}},
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("expected deadline")
			}
			remaining := time.Until(deadline)
			if remaining <= 4*time.Second || remaining > DefaultInvokeTimeout {
				t.Fatalf("unexpected default timeout remaining: %s", remaining)
			}
			return nil
		},
	}

	err := invoker.Invoke(context.Background(), &DNS{
		Service:   "auth",
		Namespace: "default",
	}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestUnaryInvoker_Invoke_PreservesParentCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	invoker := &UnaryInvoker{
		Dialer: testDialer{conn: &grpc.ClientConn{}},
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			select {
			case <-ctx.Done():
				return nil
			default:
				t.Fatal("expected canceled context")
				return nil
			}
		},
	}

	err := invoker.Invoke(parent, &DNS{
		Service:   "auth",
		Namespace: "default",
	}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestNewOutgoingCallContext_CopiesMetadata(t *testing.T) {
	source := metadata.Pairs("x-request-id", "req-1")
	ctx, cancel := NewOutgoingCallContext(context.Background(), source, 0)
	defer cancel()

	source.Set("x-request-id", "req-2")

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing metadata")
	}
	if got := md.Get("x-request-id"); len(got) == 0 || got[0] != "req-1" {
		t.Fatalf("metadata should be copied before writing outgoing context: %v", got)
	}
}

func TestNewOutgoingCallContext_PreservesParentCancellationAndTimeout(t *testing.T) {
	parent, stop := context.WithCancel(context.Background())
	ctx, cancel := NewOutgoingCallContext(parent, metadata.Pairs("x-request-id", "req-1"), time.Second)
	defer cancel()
	stop()

	select {
	case <-ctx.Done():
	default:
		t.Fatal("expected child context to be canceled with parent")
	}

	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("expected timeout to add deadline")
	}
}

func TestResolveOutgoingMetadata_ReusesOutgoingContextMetadata(t *testing.T) {
	parent := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-request-id", "req-inherited"))
	md := resolveOutgoingMetadata(parent, "", "")

	if got := md.Get("x-request-id"); len(got) == 0 || got[0] != "req-inherited" {
		t.Fatalf("unexpected inherited outgoing metadata: %v", got)
	}
}

func TestResolveOutgoingMetadata_InjectsCallerServiceIdentity(t *testing.T) {
	md := resolveOutgoingMetadata(
		context.Background(),
		"auth",
		"auth-1",
	)

	if got := md.Get(constant.ServiceAppId); len(got) == 0 || got[0] != "auth" {
		t.Fatalf("unexpected service app id metadata: %v", got)
	}
	if got := md.Get(constant.ServiceInstanceId); len(got) == 0 || got[0] != "auth-1" {
		t.Fatalf("unexpected service instance id metadata: %v", got)
	}
}

func TestUnaryInvoker_Invoke_ReturnsErrorWhenDialerMissing(t *testing.T) {
	var invoker *UnaryInvoker

	err := invoker.Invoke(context.Background(), &DNS{Service: "auth", Namespace: "default"}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if err != ErrInvokerDialerIsNil {
		t.Fatalf("expected %v, got %v", ErrInvokerDialerIsNil, err)
	}
}

func TestUnaryInvoker_Invoke_ReturnsErrorWhenMethodEmpty(t *testing.T) {
	invoker := &UnaryInvoker{Dialer: testDialer{conn: &grpc.ClientConn{}}}

	err := invoker.Invoke(context.Background(), &DNS{Service: "auth", Namespace: "default"}, "", struct{}{}, &struct{}{})
	if err != ErrInvokeMethodEmpty {
		t.Fatalf("expected %v, got %v", ErrInvokeMethodEmpty, err)
	}
}

func TestUnaryInvoker_Invoke_PropagatesDialError(t *testing.T) {
	expectedErr := errors.New("dial failed")
	invoker := &UnaryInvoker{
		Dialer: testDialer{err: expectedErr},
	}

	err := invoker.Invoke(context.Background(), &DNS{Service: "auth", Namespace: "default"}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestUnaryInvoker_Invoke_UsesDefaultUnaryInvokeFuncWhenInvokeFuncMissing(t *testing.T) {
	conn, err := grpc.NewClient("passthrough:///auth.default.svc.cluster.local:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	defer func() { _ = conn.Close() }()

	invoker := &UnaryInvoker{
		Dialer: testDialer{conn: conn},
	}

	err = invoker.Invoke(context.Background(), &DNS{Service: "auth", Namespace: "default"}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if err == nil {
		t.Fatal("expected error from default grpc invoke path")
	}
}

func TestNewOutgoingCallContext_AllowsNilParentAndNilMetadata(t *testing.T) {
	ctx, cancel := NewOutgoingCallContext(nil, nil, 0)
	defer cancel()

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing metadata")
	}
	if len(md) != 0 {
		t.Fatalf("expected empty metadata, got %v", md)
	}
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("expected default timeout deadline")
	}
}
