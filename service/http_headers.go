package service

import (
	"net/http"
	"strings"

	"github.com/fireflycore/go-micro/constant"
)

// ContextHTTPHeaderKeys 返回框架拥有的 HTTP 上下文键，调用方可据此清除旧值。
func ContextHTTPHeaderKeys() []string {
	keys := append(append([]string{}, clientEnvironmentHeaders...), observedHeaders...)
	return append(keys, constant.UserAuthority, constant.ServiceAuthority, constant.AuthzSign,
		constant.AcceptLanguage, constant.InvokeMethod, constant.InvokePath,
		constant.TraceParent, constant.TraceState, constant.Baggage)
}

// ClearPrivateHTTPHeaders 移除私有或废弃身份输入，保留无关上传/协议头。
func ClearPrivateHTTPHeaders(h http.Header) {
	public := map[string]bool{}
	for _, key := range ContextHTTPHeaderKeys() {
		public[key] = true
	}
	private := map[string]bool{}
	for _, key := range []string{
		constant.UserAuthority, constant.ServiceAuthority, constant.AuthzSign,
		constant.XRealIp, constant.XForwardedFor, "authorization",
		"x-firefly-service-app-id", "x-firefly-service-instance-id",
		"x-firefly-org-ids", "x-firefly-post-ids", "x-firefly-role-ids",
		"x-firefly-invoke-app-id", "x-firefly-target-app-id",
		"x-firefly-route-method", "x-firefly-route-path", "x-firefly-target-method", "x-firefly-target-path",
		"x-firefly-api-method", "x-firefly-api-path", "x-firefly-access-method",
	} {
		private[key] = true
	}
	for key := range h {
		name := strings.ToLower(key)
		// 使用实际键删除，不能假设调用方总使用 net/http 规范化键名。
		if private[name] || (strings.HasPrefix(name, constant.HeaderPrefix) && !public[name]) {
			delete(h, key)
		}
	}
}
