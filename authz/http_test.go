package authz

import (
	"context"
	"net/http"
	"strings"
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
