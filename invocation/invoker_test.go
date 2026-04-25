package invocation

import (
	"context"
	"testing"
	"time"

	"github.com/fireflycore/go-micro/constant"
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

	err := invoker.Invoke(ctx, &ServiceDNS{
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

func TestUnaryInvoker_Invoke_WithoutExplicitOptionsDoesNotPanic(t *testing.T) {
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

func TestUnaryInvoker_Invoke_ExplicitMetadataCannotOverrideProtectedCallerFields(t *testing.T) {
	invoker := &UnaryInvoker{
		Dialer: testDialer{conn: &grpc.ClientConn{}},
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				t.Fatal("expected outgoing metadata")
			}
			if got := md.Get(constant.UserId); len(got) == 0 || got[0] != "u-incoming" {
				t.Fatalf("unexpected protected user id metadata: %v", got)
			}
			if got := md.Get("x-request-id"); len(got) == 0 || got[0] != "req-1" {
				t.Fatalf("unexpected explicit metadata: %v", got)
			}
			return nil
		},
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.UserId, "u-incoming",
	))

	err := invoker.Invoke(ctx, &ServiceDNS{
		Service:   "auth",
		Namespace: "default",
	}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{},
		WithMetadata(metadata.Pairs(
			constant.UserId, "u-explicit",
			"x-request-id", "req-1",
		)),
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestUnaryInvoker_Invoke_WithTimeoutAddsDeadline(t *testing.T) {
	invoker := &UnaryInvoker{
		Dialer: testDialer{conn: &grpc.ClientConn{}},
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("expected deadline")
			}
			if time.Until(deadline) <= 0 {
				t.Fatal("expected future deadline")
			}
			return nil
		},
	}

	err := invoker.Invoke(context.Background(), &ServiceDNS{
		Service:   "auth",
		Namespace: "default",
	}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{}, WithTimeout(200*time.Millisecond))
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

	err := invoker.Invoke(parent, &ServiceDNS{
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

func TestMergeOptionMetadata_CopiesInputs(t *testing.T) {
	base := metadata.Pairs("x-request-id", "req-1")
	extra := metadata.Pairs("x-debug", "1")

	merged := mergeOptionMetadata(base, extra)
	base.Set("x-request-id", "req-2")
	extra.Set("x-debug", "0")

	if got := merged.Get("x-request-id"); len(got) == 0 || got[0] != "req-1" {
		t.Fatalf("unexpected merged request id metadata: %v", got)
	}
	if got := merged.Get("x-debug"); len(got) == 0 || got[0] != "1" {
		t.Fatalf("unexpected merged debug metadata: %v", got)
	}
}

func TestMergeOptionMetadata_AllowsOverrideByLaterOption(t *testing.T) {
	merged := mergeOptionMetadata(
		metadata.Pairs("x-request-id", "req-1"),
		metadata.Pairs("x-request-id", "req-2"),
	)

	if got := merged.Get("x-request-id"); len(got) == 0 || got[0] != "req-2" {
		t.Fatalf("expected later metadata to override earlier value: %v", got)
	}
}

func TestPrepareOutgoingMetadata_ReusesOutgoingContextMetadata(t *testing.T) {
	parent := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-request-id", "req-inherited"))
	md := prepareOutgoingMetadata(parent, metadata.Pairs("x-debug", "1"))

	if got := md.Get("x-request-id"); len(got) == 0 || got[0] != "req-inherited" {
		t.Fatalf("unexpected inherited outgoing metadata: %v", got)
	}
	if got := md.Get("x-debug"); len(got) == 0 || got[0] != "1" {
		t.Fatalf("unexpected explicit metadata: %v", got)
	}
}
