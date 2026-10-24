package hm

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fireflycore/go-micro/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestNewAccessLoggerRecordsResponse 验证显式装配日志后仍记录真实入口与响应状态。
func TestNewAccessLoggerRecordsResponse(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	handler := NewAccessLogger(logger.NewAccessLogger(zap.New(core)))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/example", nil))
	if recorder.Code != http.StatusNoContent || logs.Len() != 1 {
		t.Fatalf("unexpected response or access log count: status=%d logs=%d", recorder.Code, logs.Len())
	}
	fields := logs.All()[0].ContextMap()
	if fields["method"] != http.MethodGet || fields["path"] != "/example" || fields["status"] != int64(http.StatusNoContent) {
		t.Fatalf("unexpected access log fields: %v", fields)
	}
}

// TestNewAccessLoggerRequiresLogger 验证 nil 不作为静默关闭 HTTP 访问日志的配置。
func TestNewAccessLoggerRequiresLogger(t *testing.T) {
	handler := NewAccessLogger(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer func() {
		if recover() == nil {
			t.Fatal("nil logger was silently accepted")
		}
	}()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/example", nil))
}
