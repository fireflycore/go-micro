package gm

import (
	"context"
	"testing"

	"github.com/fireflycore/go-micro/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
)

func TestNewAccessLoggerSkipsHealthCheckByDefault(t *testing.T) {
	baseCore, observed := observer.New(zapcore.InfoLevel)
	accessLogger := logger.NewAccessLogger(zap.New(baseCore))
	interceptor := NewAccessLogger(accessLogger)

	handlerCalled := false
	_, err := interceptor(
		context.Background(),
		map[string]string{"k": "v"},
		&grpc.UnaryServerInfo{FullMethod: grpcHealthCheckFullMethod},
		func(ctx context.Context, req any) (any, error) {
			handlerCalled = true
			return map[string]string{"status": "ok"}, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Fatalf("expected handler to be called")
	}
	if got := observed.Len(); got != 0 {
		t.Fatalf("expected no access logs for health check, got %d", got)
	}
}

func TestNewAccessLoggerSkipsConfiguredMethod(t *testing.T) {
	baseCore, observed := observer.New(zapcore.InfoLevel)
	accessLogger := logger.NewAccessLogger(zap.New(baseCore))
	interceptor := NewAccessLogger(accessLogger, AccessLoggerOptions{
		SkipMethods: []string{"/example.Service/Ping"},
	})

	handlerCalled := false
	_, err := interceptor(
		context.Background(),
		map[string]string{"k": "v"},
		&grpc.UnaryServerInfo{FullMethod: "/example.Service/Ping"},
		func(ctx context.Context, req any) (any, error) {
			handlerCalled = true
			return map[string]string{"status": "ok"}, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Fatalf("expected handler to be called")
	}
	if got := observed.Len(); got != 0 {
		t.Fatalf("expected configured method to be skipped, got %d logs", got)
	}
}

func TestNewAccessLoggerLogsNonSkippedMethod(t *testing.T) {
	baseCore, observed := observer.New(zapcore.InfoLevel)
	accessLogger := logger.NewAccessLogger(zap.New(baseCore))
	interceptor := NewAccessLogger(accessLogger)

	handlerCalled := false
	_, err := interceptor(
		context.Background(),
		map[string]string{"k": "v"},
		&grpc.UnaryServerInfo{FullMethod: "/example.Service/Get"},
		func(ctx context.Context, req any) (any, error) {
			handlerCalled = true
			return map[string]string{"status": "ok"}, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Fatalf("expected handler to be called")
	}
	if got := observed.Len(); got != 1 {
		t.Fatalf("expected one access log for non-skipped method, got %d", got)
	}
}
