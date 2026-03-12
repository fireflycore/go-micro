package hm

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
)

var RequestMethod = map[string]uint32{
	"GET":    1,
	"POST":   2,
	"PUT":    3,
	"DELETE": 4,
}

// HttpStatusResponseWriter 它主要由中间件（例如访问日志记录器）使用
type HttpStatusResponseWriter struct {
	http.ResponseWriter
	status int
	resp   *bytes.Buffer
}

func (w *HttpStatusResponseWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
	w.status = code
}

func (w *HttpStatusResponseWriter) Write(b []byte) (int, error) {
	w.resp.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *HttpStatusResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *HttpStatusResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return h.Hijack()
}

func (w *HttpStatusResponseWriter) Push(target string, opts *http.PushOptions) error {
	p, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return p.Push(target, opts)
}
