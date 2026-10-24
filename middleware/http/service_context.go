package hm

import (
	"net"
	"net/http"
	"strings"

	"github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc/metadata"
)

// ServiceContextOptions 配置 HTTP 业务入口的本地身份与受控代理策略。
type ServiceContextOptions struct {
	// ServiceAppId 是本地服务应用，用于与签名投递绑定比较。
	ServiceAppId string
	// ServiceInstanceId 是本地实例，不从请求头读取。
	ServiceInstanceId string
	// AuthzVerification 是业务入口必需的本地验签配置，只有显式 SkipEntries 可豁免。
	AuthzVerification *service.AuthzSignVerificationOptions
	// SkipEntries 只精确匹配方法和路径，不按前缀跳过。
	SkipEntries map[string]map[string]bool
	// TrustedClientIP 必须先验证连接对端再读取代理头，默认只取 RemoteAddr。
	TrustedClientIP func(*http.Request) string
}

// NewServiceContextMiddleware 捕获实际 HTTP 方法/路径，不允许签名反填真实入口。
func NewServiceContextMiddleware(options ServiceContextOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			md := metadata.MD{}
			for k, v := range r.Header {
				// 统一大小写后合并，后续收集器才能发现重复标量头。
				key := strings.ToLower(k)
				md[key] = append(md[key], v...)
			}

			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			if options.TrustedClientIP != nil {
				ip = options.TrustedClientIP(r)
			}

			build := service.BuildContextOptions{
				ServiceAppId: options.ServiceAppId, ServiceInstanceId: options.ServiceInstanceId,
				ServiceMethod: r.Method, ServicePath: r.URL.Path, ClientIp: ip,
			}

			if !options.SkipEntries[r.Method][r.URL.Path] {
				// 漏装验签配置不是匿名或公共接口豁免，不能静默放行业务请求。
				if options.AuthzVerification == nil {
					http.Error(w, "authz verification is not configured", http.StatusInternalServerError)
					return
				}
				build.AuthzVerification = options.AuthzVerification
			}

			ctx, err := service.NewIncomingContext(r.Context(), md, build)
			if err != nil {
				http.Error(w, "invalid service context", http.StatusUnauthorized)
				return
			}

			copy := r.Clone(ctx)

			// 清理副本上的凭证及废弃头，保留原请求和无关上传/协议头。
			service.ClearPrivateHTTPHeaders(copy.Header)
			next.ServeHTTP(w, copy)
		})
	}
}
