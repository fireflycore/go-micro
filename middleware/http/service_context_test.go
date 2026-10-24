package hm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/internal/contexttest"
	"github.com/fireflycore/go-micro/service"
	"go.opentelemetry.io/otel/trace"
)

// TestHTTPActualBindingAndPrivateHeaders 验证真实 HTTP 入口绑定，以及业务可见请求头的私有输入清理。
func TestHTTPActualBindingAndPrivateHeaders(t *testing.T) {
	claims := contexttest.Claims("service", true)
	claims["service_method"], claims["service_path"] = "POST", "/v1/items"
	options := contexttest.Options()
	called := 0
	handler := NewServiceContextMiddleware(ServiceContextOptions{ServiceAppId: "B", AuthzVerification: &options})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		c := service.MustFromContext(r.Context())
		if c.ServiceMethod != "POST" || c.ServicePath != "/v1/items" || c.ClientIp != "127.0.0.1" || c.UserContext.AppId != "login-app" {
			t.Fatal("wrong entry")
		}
		if r.Header.Get(constant.UserAuthority) != "" || r.Header.Get(constant.AuthzSign) != "" || r.Header.Get(constant.XRealIp) != "" || r.Header.Get("x-firefly-unknown") != "" {
			t.Fatal("private input exposed")
		}
		if r.Header.Get("content-md5") != "keep" || !trace.SpanContextFromContext(r.Context()).IsValid() {
			t.Fatal("lost protocol header/span")
		}
		for k := range r.Header {
			if strings.EqualFold(k, "authorization") || strings.EqualFold(k, constant.AuthzSign) {
				t.Fatal("noncanonical credential header exposed")
			}
		}
	}))
	r := httptest.NewRequest("POST", "http://local/v1/items", nil)
	r.RemoteAddr = "127.0.0.1:123"
	r.Header.Set(constant.AuthzSign, contexttest.Sign(claims))
	r.Header.Set(constant.UserAuthority, "secret")
	r.Header.Set(constant.XRealIp, "forged")
	r.Header.Set("x-firefly-unknown", "drop")
	r.Header.Set("content-md5", "keep")
	r.Header.Set("traceparent", "00-1234567890abcdef1234567890abcdef-1234567890abcdef-01")
	r.Header["authorization"] = []string{"raw-secret"}
	handler.ServeHTTP(httptest.NewRecorder(), r)
	// 不同大小写的同名签名头也属于重复输入，不能绕过标量校验。
	duplicate := r.Clone(context.Background())
	duplicate.Header[constant.AuthzSign] = []string{contexttest.Sign(claims)}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, duplicate)
	if w.Code != 401 || called != 1 {
		t.Fatal("accepted duplicate case-variant proof headers")
	}
	// 合法签名投递到其他路径必须失败，且不能反向修改调用方持有的请求。
	r = r.Clone(context.Background())
	r.URL.Path = "/wrong"
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 401 || called != 1 || r.Header.Get(constant.UserAuthority) != "secret" {
		t.Fatal("binding or input mutation")
	}
}

// TestHTTPMissingVerificationFailsClosed 区分验签装配缺失与精确配置的独立入口。
func TestHTTPMissingVerificationFailsClosed(t *testing.T) {
	for _, sample := range []struct {
		method     string // 实际 HTTP 方法，不允许仅按路径扩大豁免。
		path       string // 实际路径，跳过规则不使用前缀匹配。
		wantStatus int    // 业务入口装配缺失返回 500，独立入口正常执行。
	}{
		{"POST", "/v1/items", http.StatusInternalServerError},
		{"POST", "/health", http.StatusInternalServerError},
		{"GET", "/health/detail", http.StatusInternalServerError},
		{"GET", "/health", http.StatusNoContent},
	} {
		t.Run(sample.method+sample.path, func(t *testing.T) {
			called := false
			handler := NewServiceContextMiddleware(ServiceContextOptions{
				ServiceAppId: "B", SkipEntries: map[string]map[string]bool{"GET": {"/health": true}},
			})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				c := service.MustFromContext(r.Context())
				if c.UserContext != nil || c.AuthzSignContext != nil || c.InvokeAppId != "" {
					t.Fatal("independent entry created trusted identity")
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			request := httptest.NewRequest(sample.method, "http://local"+sample.path, nil)
			request.Header.Set(constant.UserId, "forged")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != sample.wantStatus || called != (sample.wantStatus == http.StatusNoContent) {
				t.Fatalf("status=%d handler_called=%v", response.Code, called)
			}
		})
	}
}
