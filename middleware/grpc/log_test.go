package gm

import (
	"context"
	"testing"

	"github.com/fireflycore/go-micro/logger"
	"github.com/fireflycore/go-micro/werror"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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

func TestNewAccessLoggerLogsStructuredErrorIdentity(t *testing.T) {
	baseCore, observed := observer.New(zapcore.InfoLevel)
	accessLogger := logger.NewAccessLogger(zap.New(baseCore))
	interceptor := NewAccessLogger(accessLogger)
	definition := werror.Definition{
		Code:    codes.Unauthenticated,
		Domain:  "lhdht.auth",
		Reason:  "TOKEN_INVALID",
		Message: "令牌已失效",
	}

	_, err := interceptor(
		context.Background(),
		map[string]string{"k": "v"},
		&grpc.UnaryServerInfo{FullMethod: "/example.Service/Get"},
		func(ctx context.Context, req any) (any, error) {
			return nil, definition.New(werror.WithDetail("token", "sensitive"))
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := observed.Len(); got != 1 {
		t.Fatalf("expected one access log, got %d", got)
	}
	fields := observed.All()[0].ContextMap()
	if fields["error_domain"] != definition.Domain || fields["error_reason"] != definition.Reason {
		t.Fatalf("unexpected error identity fields: %+v", fields)
	}
	if _, exists := fields["error_metadata"]; exists {
		t.Fatal("error metadata must not be logged by default")
	}
}

// TestNewAccessLoggerRequiresLogger 验证缺少日志依赖时不会静默关闭普通 RPC 的访问日志。
func TestNewAccessLoggerRequiresLogger(t *testing.T) {
	interceptor := NewAccessLogger(nil)
	defer func() {
		if recover() == nil {
			t.Fatal("nil logger was silently accepted")
		}
	}()
	_, _ = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/example.Service/Get"},
		func(context.Context, any) (any, error) { return nil, nil })
}

// TestNewAccessLoggerRequiresInfo 验证错误调用缺少 RPC 信息时，不会先执行 handler 再失败。
func TestNewAccessLoggerRequiresInfo(t *testing.T) {
	interceptor := NewAccessLogger(logger.NewAccessLogger(zap.NewNop()))
	handlerCalled := false
	defer func() {
		if recover() == nil {
			t.Fatal("nil RPC info was silently accepted")
		}
		if handlerCalled {
			t.Fatal("handler executed before missing RPC info was exposed")
		}
	}()
	_, _ = interceptor(context.Background(), nil, nil, func(context.Context, any) (any, error) {
		handlerCalled = true
		return nil, nil
	})
}
