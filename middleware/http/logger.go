package hm

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/logger"
	"go.uber.org/zap"
)

// NewAccessLogger 访问日志中间件
func NewAccessLogger(log *logger.Core) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if log == nil {
				next.ServeHTTP(writer, request)
				return
			}

			start := time.Now()

			var req []byte
			if request.Method != http.MethodGet {
				req, _ = io.ReadAll(request.Body)
				request.Body = io.NopCloser(bytes.NewBuffer(req))
			} else {
				query := request.URL.RawQuery
				query, _ = url.QueryUnescape(query)
				split := strings.Split(query, "&")
				m := make(map[string]string)
				for _, v := range split {
					kv := strings.Split(v, "=")
					if len(kv) == 2 {
						m[kv[0]] = kv[1]
					}
				}
				req, _ = json.Marshal(&m)
			}

			sw := &HttpStatusResponseWriter{writer, http.StatusOK, &bytes.Buffer{}}

			next.ServeHTTP(sw, request)

			status := sw.status
			elapsed := time.Since(start)

			method, ok := RequestMethod[request.Method]
			if !ok {
				method = 0
			}

			clientType, ce := strconv.ParseInt(request.Header.Get(constant.ClientType), 10, 32)
			if ce != nil {
				clientType = 0
			}
			systemType, se := strconv.ParseInt(request.Header.Get(constant.SystemType), 10, 32)
			if se != nil {
				systemType = 0
			}

			fields := make([]zap.Field, 0, 24)
			fields = append(fields,
				zap.String("log_type", "access"),
				zap.String("protocol", "http"),
				zap.Uint32("method", method),
				zap.String("path", request.URL.Path),
				zap.Uint64("duration", uint64(elapsed.Microseconds())),
				zap.Uint32("status", uint32(status)),
				zap.String("request", string(req)),
				zap.String("response", sw.resp.String()),
				zap.Uint32("client_type", uint32(clientType)),
				zap.String("client_name", request.Header.Get(constant.ClientName)),
				zap.String("client_version", request.Header.Get(constant.ClientVersion)),
				zap.Uint32("system_type", uint32(systemType)),
				zap.String("system_name", request.Header.Get(constant.SystemName)),
				zap.String("system_version", request.Header.Get(constant.SystemVersion)),
				zap.String("app_version", request.Header.Get(constant.AppVersion)),
				zap.String("source_ip", request.Header.Get(constant.SourceIp)),
				zap.String("client_ip", request.Header.Get(constant.ClientIp)),
			)

			if status >= 400 {
				log.Warn(request.Context(), "http access log", fields...)
			} else {
				log.Info(request.Context(), "http access log", fields...)
			}
		})
	}
}
