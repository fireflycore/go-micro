package hm

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
)

// HttpStatusResponseWriter 它主要由中间件（例如访问日志记录器）使用
type HttpStatusResponseWriter struct {
	// ResponseWriter 委托原始响应写入，保留底层连接能力。
	http.ResponseWriter
	// status 保存显式写入的状态码，访问日志入口将初值设为 200。
	status int
	// resp 仅在显式提供缓冲时收集响应体，默认访问日志不启用它。
	resp *bytes.Buffer
}

// WriteHeader 记录实际状态，不额外展开响应内容。
func (w *HttpStatusResponseWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
	w.status = code
}

// Write 保持原始写入结果；未装配缓冲时不复制响应体。
func (w *HttpStatusResponseWriter) Write(b []byte) (int, error) {
	if w.resp != nil {
		w.resp.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

// Flush 保留支持流式响应的底层能力。
func (w *HttpStatusResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack 透传连接接管能力，底层不支持时返回标准错误。
func (w *HttpStatusResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return h.Hijack()
}

// Push 保留底层 HTTP/2 推送接口，不更改协议行为。
func (w *HttpStatusResponseWriter) Push(target string, opts *http.PushOptions) error {
	p, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return p.Push(target, opts)
}
