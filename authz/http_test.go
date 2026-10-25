package authz

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc/metadata"
)

// roundTripFunc 在不访问网络的情况下检查最终发出的 HTTP 请求。
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestHTTPTransportFreshWhitelist 验证精确白名单、大小写变体清理和原请求隔离。
func TestHTTPTransportFreshWhitelist(t *testing.T) {
	ctx, err := service.NewIncomingContext(context.Background(), metadata.Pairs(constant.UserAuthority, "user-token"), service.BuildContextOptions{ServiceMethod: "GRPC", ServicePath: "/b"})
	if err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequestWithContext(ctx, "POST", "http://local/c", nil)
	r.Header.Set("x-firefly-unknown", "drop")
	r.Header.Set(constant.AuthzSign, "old-proof")
	r.Header.Set("content-md5", "keep")
	r.Header.Set(constant.AcceptLanguage, "drop")
	// 直接写入 Header 映射可能绕过 net/http 的键名规范化，仍须清除旧身份输入。
	r.Header["authorization"] = []string{"old-token"}
	r.Header[constant.AuthzSign] = []string{"lowercase-proof"}
	r.Header[constant.ServiceAuthority] = []string{"old-service-token"}
	r.Header["x-firefly-unknown"] = []string{"lowercase-unknown"}
	r.Header["accept-language"] = []string{"lowercase-language"}
	p := fixedServiceAuthorityProvider("service-token")
	transport := AuthorityTransport{Provider: p, Base: roundTripFunc(func(out *http.Request) (*http.Response, error) {
		for k, values := range out.Header {
			for _, value := range values {
				if strings.HasPrefix(value, "old-") || strings.HasPrefix(value, "lowercase-") {
					t.Fatalf("leaked noncanonical header: %s", k)
				}
			}
		}
		if out.Header.Get("x-firefly-unknown") != "" || out.Header.Get(constant.AuthzSign) != "" || out.Header.Get(constant.AcceptLanguage) != "" {
			t.Fatal("leaked context")
		}
		if out.Header.Get(constant.UserAuthority) != "user-token" || out.Header.Get(constant.ServiceAuthority) != "service-token" ||
			out.Header.Get(constant.InvokePath) != "/b" || out.Header.Get("content-md5") != "keep" {
			t.Fatal("incorrect whitelist")
		}
		return &http.Response{StatusCode: 200}, nil
	})}
	if _, err = transport.RoundTrip(r); err != nil {
		t.Fatal(err)
	}
	if r.Header.Get(constant.AuthzSign) != "old-proof" {
		t.Fatal("mutated request")
	}
}

// TestHTTPTransportKeepsUTF8Context 不把 gRPC 的 ASCII 限制扩散到 HTTP，也不改变输入原文。
func TestHTTPTransportKeepsUTF8Context(t *testing.T) {
	ctx, err := service.NewIncomingContext(context.Background(), metadata.Pairs(constant.ClientName, "中文客户端"),
		service.BuildContextOptions{ServiceAppId: "B", ServiceMethod: "POST", ServicePath: "/入口"})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(ctx, "GET", "http://local/target", nil)
	if err != nil {
		t.Fatal(err)
	}
	transport := AuthorityTransport{
		Provider: fixedServiceAuthorityProvider("B-token"),
		Base: roundTripFunc(func(out *http.Request) (*http.Response, error) {
			if out.Header.Get(constant.ClientName) != "中文客户端" || out.Header.Get(constant.InvokePath) != "/入口" {
				t.Fatal("HTTP context encoding was changed")
			}
			return &http.Response{StatusCode: http.StatusOK}, nil
		}),
	}
	if _, err := transport.RoundTrip(request); err != nil {
		t.Fatal(err)
	}
}

// countingHTTPAuthorityProvider 用调用次数验证危险重定向在取凭证之前被拒绝。
type countingHTTPAuthorityProvider struct {
	// calls 只在单个测试的顺序 HTTP 调用中累加，不与其他用例共享。
	calls int
}

func (p *countingHTTPAuthorityProvider) ServiceAuthority(context.Context) (string, error) {
	p.calls++
	return "service-token", nil
}

// TestHTTPTransportRedirectBoundary 经真实 http.Client 重定向流程检查最终发送边界，不访问网络。
func TestHTTPTransportRedirectBoundary(t *testing.T) {
	cases := []struct {
		// name 标识同源或越界场景。
		name string
		// start 是调用方显式选择的初始内部目标。
		start string
		// locations 按顺序模拟各次响应的 Location。
		locations []string
		// status 是重定向状态码，零值使用 302。
		status int
		// blocked 表示最后一次重定向必须在取凭证和发送前失败。
		blocked bool
	}{
		{name: "relative", start: "https://internal.invalid/start", locations: []string{"/next"}},
		{name: "same_origin_chain", start: "https://internal.invalid/start", locations: []string{"/next", "/final"}},
		{name: "host_case", start: "https://internal.invalid/start", locations: []string{"https://INTERNAL.invalid/next"}},
		{name: "https_default_port", start: "https://internal.invalid/start", locations: []string{"https://internal.invalid:443/next"}},
		{name: "http_default_port", start: "http://internal.invalid:80/start", locations: []string{"http://internal.invalid/next"}},
		{name: "ipv6", start: "https://[::1]/start", locations: []string{"https://[::1]:443/next"}},
		{name: "other_host", start: "https://internal.invalid/start", locations: []string{"https://external.invalid/collect"}, blocked: true},
		{name: "subdomain", start: "https://internal.invalid/start", locations: []string{"https://child.internal.invalid/collect"}, blocked: true},
		{name: "other_port", start: "https://internal.invalid/start", locations: []string{"https://internal.invalid:8443/collect"}, blocked: true},
		{name: "downgrade", start: "https://internal.invalid/start", locations: []string{"http://internal.invalid/collect"}, blocked: true},
		{name: "upgrade", start: "http://internal.invalid/start", locations: []string{"https://internal.invalid/collect"}, blocked: true},
		{name: "chain_leaves_origin", start: "https://internal.invalid/start", locations: []string{"/next", "https://external.invalid/collect"}, blocked: true},
		{name: "same_origin_301", start: "https://internal.invalid/start", locations: []string{"/next"}, status: http.StatusMovedPermanently},
		{name: "cross_origin_301", start: "https://internal.invalid/start", locations: []string{"https://external.invalid/collect"}, status: http.StatusMovedPermanently, blocked: true},
		{name: "same_origin_303", start: "https://internal.invalid/start", locations: []string{"/next"}, status: http.StatusSeeOther},
		{name: "cross_origin_303", start: "https://internal.invalid/start", locations: []string{"https://external.invalid/collect"}, status: http.StatusSeeOther, blocked: true},
		{name: "same_origin_307", start: "https://internal.invalid/start", locations: []string{"/next"}, status: http.StatusTemporaryRedirect},
		{name: "cross_origin_307", start: "https://internal.invalid/start", locations: []string{"https://external.invalid/collect"}, status: http.StatusTemporaryRedirect, blocked: true},
		{name: "same_origin_308", start: "https://internal.invalid/start", locations: []string{"/next"}, status: http.StatusPermanentRedirect},
		{name: "cross_origin_308", start: "https://internal.invalid/start", locations: []string{"https://external.invalid/collect"}, status: http.StatusPermanentRedirect, blocked: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, err := service.NewIncomingContext(context.Background(), metadata.Pairs(constant.UserAuthority, "user-token"),
				service.BuildContextOptions{ServiceAppId: "B", ServiceMethod: "GRPC", ServicePath: "/b"})
			if err != nil {
				t.Fatal(err)
			}
			provider := &countingHTTPAuthorityProvider{}
			sent := 0
			client := &http.Client{Transport: &AuthorityTransport{
				Provider: provider,
				Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					sent++
					// 仅允许初始请求或已通过同源核对的下一跳到达实际传输器。
					if tc.blocked && sent > len(tc.locations) {
						t.Fatal("redirect escaped the trusted origin")
					}
					if r.Header.Get(constant.UserAuthority) != "user-token" || r.Header.Get(constant.ServiceAuthority) != "service-token" {
						t.Fatal("allowed request lost its authority")
					}
					response := &http.Response{
						StatusCode: http.StatusOK, Header: make(http.Header),
						Body: io.NopCloser(strings.NewReader("")), Request: r,
					}
					if sent <= len(tc.locations) {
						response.StatusCode = tc.status
						if response.StatusCode == 0 {
							response.StatusCode = http.StatusFound
						}
						response.Header.Set("Location", tc.locations[sent-1])
					}
					return response, nil
				}),
			}}
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, tc.start, nil)
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.Do(request)
			wantSent := len(tc.locations) + 1
			if tc.blocked {
				wantSent--
				if !errors.Is(err, ErrAuthorityRedirectNotAllowed) {
					t.Fatalf("got %v, want redirect rejection", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if err := response.Body.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if sent != wantSent || provider.calls != wantSent {
				t.Fatalf("sent=%d provider=%d, want %d", sent, provider.calls, wantSent)
			}
			if request.Header.Get(constant.UserAuthority) != "" || request.Header.Get(constant.ServiceAuthority) != "" {
				t.Fatal("transport mutated the original request")
			}
		})
	}
}

// TestHTTPTransportRejectsUnknownRedirectOrigin 不使用当前目标反填缺失的上一跳事实。
func TestHTTPTransportRejectsUnknownRedirectOrigin(t *testing.T) {
	for _, previous := range []*http.Request{nil, {}} {
		provider := &countingHTTPAuthorityProvider{}
		transport := &AuthorityTransport{
			Provider: provider,
			Base: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("unverifiable redirect reached the base transport")
				return nil, nil
			}),
		}
		request, err := http.NewRequest(http.MethodGet, "https://internal.invalid/next", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Response = &http.Response{Request: previous}
		if _, err := transport.RoundTrip(request); !errors.Is(err, ErrAuthorityRedirectNotAllowed) {
			t.Fatalf("got %v, want redirect rejection", err)
		}
		if provider.calls != 0 {
			t.Fatal("unverifiable redirect fetched an authority")
		}
	}
}

// TestHTTPTransportLoopbackRedirect 使用真实 HTTP 收发验证同源链可达，跨端口目标收不到请求。
func TestHTTPTransportLoopbackRedirect(t *testing.T) {
	var received atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()

	var sourceCalls atomic.Int32
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceCalls.Add(1)
		if r.Header.Get(constant.UserAuthority) != "user-token" || r.Header.Get(constant.ServiceAuthority) != "service-token" {
			t.Error("same-origin request lost its authority")
		}
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/next", http.StatusFound)
			return
		}
		http.Redirect(w, r, destination.URL+"/collect", http.StatusFound)
	}))
	defer source.Close()

	ctx, err := service.NewIncomingContext(context.Background(), metadata.Pairs(constant.UserAuthority, "user-token"),
		service.BuildContextOptions{ServiceAppId: "B", ServiceMethod: "GRPC", ServicePath: "/b"})
	if err != nil {
		t.Fatal(err)
	}
	provider := &countingHTTPAuthorityProvider{}
	client := &http.Client{Transport: &AuthorityTransport{Provider: provider}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL+"/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(request); !errors.Is(err, ErrAuthorityRedirectNotAllowed) {
		t.Fatalf("got %v, want redirect rejection", err)
	}
	if sourceCalls.Load() != 2 || provider.calls != 2 || received.Load() != 0 {
		t.Fatalf("source=%d provider=%d destination=%d, want 2/2/0", sourceCalls.Load(), provider.calls, received.Load())
	}
}
