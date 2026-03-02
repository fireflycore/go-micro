package hm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/logger"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// NewAccessLogger 访问日志中间件
// handle 接收两类日志：b 为结构化 JSON，msg 为人类可读文本行；
// 字段包含 path/request/response/duration/status/trace_id 等，便于统一采集。
func NewAccessLogger(handle func(b []byte, msg string)) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
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

			log := &logger.AccessLogger{}
			log.ParentId = request.Header.Get(constant.ParentId)
			log.TraceId = request.Header.Get(constant.TraceId)
			log.SpanId = request.Header.Get(constant.SpanId)

			method, ok := RequestMethod[request.Method]
			if !ok {
				method = 0
			}
			log.Method = method
			log.Path = request.URL.Path

			log.Request = string(req)
			log.Response = sw.resp.String()

			clientType, ce := strconv.ParseInt(request.Header.Get(constant.ClientType), 10, 32)
			if ce != nil {
				clientType = 0
			}
			systemType, se := strconv.ParseInt(request.Header.Get(constant.SystemType), 10, 32)
			if se != nil {
				systemType = 0
			}

			log.ClientType = uint32(clientType)
			log.ClientName = request.Header.Get(constant.ClientName)
			log.ClientVersion = request.Header.Get(constant.ClientVersion)

			log.SystemType = uint32(systemType)
			log.SystemName = request.Header.Get(constant.SystemName)
			log.SystemVersion = request.Header.Get(constant.SystemVersion)

			log.AppVersion = request.Header.Get(constant.AppVersion)

			log.SourceIp = request.Header.Get(constant.SourceIp)
			log.ClientIp = request.Header.Get(constant.ClientIp)

			log.Status = uint32(status)
			log.Duration = uint64(elapsed.Microseconds())

			b, _ := json.Marshal(log)

			handle(b, fmt.Sprintf(
				"[%s] [%s]:[%s] [%s]-[%d] [SourceIp:%s->ClientIp:%s]\n",
				time.Now().Format(time.DateTime),
				request.Method,
				request.URL.Path,
				elapsed.String(),
				status,
				log.SourceIp,
				log.ClientIp,
			))
		})
	}
}
