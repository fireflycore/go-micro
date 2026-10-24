package authz

import (
	"net/http"
	"strings"

	"github.com/fireflycore/go-micro/service"
)

// AuthorityTransport 在 HTTP 出站边界重建上下文，不修改调用方持有的请求。
type AuthorityTransport struct {
	// Base 是实际传输器，未指定时使用 http.DefaultTransport。
	Base http.RoundTripper
	// Provider 必须提供当前服务凭证，缺失或失败时不发送请求。
	Provider ServiceAuthorityProvider
}

// RoundTrip 保留当次显式协议/上传头，但清除旧身份、证明和遥测后重新注入。
func (t *AuthorityTransport) RoundTrip(r *http.Request) (*http.Response, error) {
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
