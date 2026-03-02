package hm

import (
	"bytes"
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
