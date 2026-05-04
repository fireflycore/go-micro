package logger

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestAccessLoggerCallerPointsToBusinessCallSite(t *testing.T) {
	baseCore, observed := observer.New(zapcore.InfoLevel)
	baseLogger := zap.New(baseCore, zap.AddCaller())
	log := NewAccessLogger(baseLogger)

	writeAccessLog(log)

	entries := observed.All()
	if len(entries) != 1 {
		t.Fatalf("expected one log entry, got %d", len(entries))
	}

	if strings.HasSuffix(entries[0].Caller.File, "logger/core_access.go") {
		t.Fatalf("unexpected wrapped caller location: %s", entries[0].Caller.File)
	}
	if !strings.HasSuffix(entries[0].Caller.File, "logger/caller_test.go") {
		t.Fatalf("expected caller to point to test call site, got %s", entries[0].Caller.File)
	}
}

func TestServerLoggerCallerPointsToBusinessCallSite(t *testing.T) {
	baseCore, observed := observer.New(zapcore.InfoLevel)
	baseLogger := zap.New(baseCore, zap.AddCaller())
	log := NewServerLogger(baseLogger)

	writeServerLog(log)

	entries := observed.All()
	if len(entries) != 1 {
		t.Fatalf("expected one log entry, got %d", len(entries))
	}

	if strings.HasSuffix(entries[0].Caller.File, "logger/core_server.go") {
		t.Fatalf("unexpected wrapped caller location: %s", entries[0].Caller.File)
	}
	if !strings.HasSuffix(entries[0].Caller.File, "logger/caller_test.go") {
		t.Fatalf("expected caller to point to test call site, got %s", entries[0].Caller.File)
	}
}

func writeAccessLog(log *AccessLogger) {
	log.WithContextInfo(context.Background(), "access")
}

func writeServerLog(log *ServerLogger) {
	log.WithContextInfo(context.Background(), "server")
}
