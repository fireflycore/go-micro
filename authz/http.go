package authz

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/fireflycore/go-micro/service"
)

// ErrAuthorityRedirectNotAllowed 表示重定向越过原目标同源边界，或缺少可核对的上一跳请求。
var ErrAuthorityRedirectNotAllowed = errors.New("authz authority redirect is not allowed")

// AuthorityTransport 在 HTTP 出站边界重建上下文，不修改调用方持有的请求。
// 仅用于显式选择的内部目标；自动重定向必须保持协议、主机和有效端口相同。
type AuthorityTransport struct {
	// Base 是实际传输器，未指定时使用 http.DefaultTransport。
	Base http.RoundTripper
	// Provider 必须提供当前服务凭证，缺失或失败时不发送请求。
	Provider ServiceAuthorityProvider
}

// RoundTrip 保留当次显式协议/上传头，但清除旧身份、证明和遥测后重新注入。
func (t *AuthorityTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Response != nil {
		// Response 由 http.Client 在重定向时设置；必须先核对目标，再获取或注入凭证。
		previous := r.Response.Request
		if previous == nil || previous.URL == nil || !sameHTTPOrigin(previous.URL, r.URL) {
			return nil, ErrAuthorityRedirectNotAllowed
		}
	}

	target := service.OutgoingTarget{TargetMethod: r.Method, TargetPath: r.URL.Path}

	out, err := buildOutgoingAuthorityContext(r.Context(), target, t.Provider)
	if err != nil {
		return nil, err
	}

	copy := r.Clone(r.Context())
	// 克隆 Header 后再清洗，避免并发调用共享的原请求被污染。
	service.ClearPrivateHTTPHeaders(copy.Header)

	contextKeys := map[string]bool{}
	for _, key := range service.ContextHTTPHeaderKeys() {
		contextKeys[key] = true
	}

	for key := range copy.Header {
		name := strings.ToLower(key)
		// 按原始 map key 删除，避免非规范大小写键绕过 http.Header.Del。
		if strings.HasPrefix(name, "x-firefly-") || contextKeys[name] {
			delete(copy.Header, key)
		}
	}

	for k, v := range out.Metadata() {
		// 只写入框架刚构建的精确白名单，不按 Firefly 前缀整体放行。
		for _, value := range v {
			copy.Header.Add(k, value)
		}
	}

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	return base.RoundTrip(copy)
}

// sameHTTPOrigin 不放行子域或协议升级/降级；显式默认端口与省略端口视为同源。
func sameHTTPOrigin(a, b *url.URL) bool {
	if !strings.EqualFold(a.Scheme, b.Scheme) || !strings.EqualFold(a.Hostname(), b.Hostname()) {
		return false
	}

	port := func(u *url.URL) string {
		if value := u.Port(); value != "" {
			return value
		}
		switch strings.ToLower(u.Scheme) {
		case "http":
			return "80"
		case "https":
			return "443"
		default:
			return ""
		}
	}

	return port(a) == port(b)
}
