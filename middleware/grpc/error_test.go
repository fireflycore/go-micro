package gm

import (
	"context"
	"errors"
	"testing"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestValidationErrorToInvalidArgumentInterceptor_MapsValidationError(t *testing.T) {
	interceptor := ValidationErrorToInvalidArgument()

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(ctx context.Context, req any) (any, error) {
		return nil, &protovalidate.ValidationError{}
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T: %v", err, err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected %v, got %v (%s)", codes.InvalidArgument, st.Code(), st.Message())
	}
}

func TestValidationErrorToInvalidArgumentInterceptor_PassesThroughOtherErrors(t *testing.T) {
	interceptor := ValidationErrorToInvalidArgument()
	origErr := errors.New("x")

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(ctx context.Context, req any) (any, error) {
		return nil, origErr
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, origErr) {
		t.Fatalf("expected original error, got %T: %v", err, err)
	}
}
