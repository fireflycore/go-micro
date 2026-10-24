package hm

import (
	"net/http"
	"time"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/logger"
	"go.uber.org/zap"
)

// NewAccessLogger 记录实际 HTTP 入口与结构化可信身份，不读取原始 token 或请求体。
// log 必须非 nil；关闭访问日志应由装配层明确不安装中间件。
func NewAccessLogger(log *logger.AccessLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &HttpStatusResponseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(sw, r)

			fields := []zap.Field{
				zap.String("log_type", "access"),
				zap.String("protocol", "http"),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Uint64("duration", uint64(time.Since(start).Microseconds())),
				zap.Int("status", sw.status),
			}
			fields = append(fields, logger.ContextFields(r.Context())...)

			if sw.status >= 500 {
				log.WithContextError(r.Context(), constant.HttpAccessLog, fields...)
			} else if sw.status >= 400 {
				log.WithContextWarn(r.Context(), constant.HttpAccessLog, fields...)
			} else {
				log.WithContextInfo(r.Context(), constant.HttpAccessLog, fields...)
			}
		})
	}
}
