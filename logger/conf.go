package logger

// Conf 是 logger 的配置项
// - Console：是否启用控制台输出
// - Remote：是否启用远端输出（需要同时提供 handle 才会生效）
type Conf struct {
	Console bool `json:"console"`
	Remote  bool `json:"remote"`

	handle func(b []byte)
}

// WithHandle 设置远端输出回调
func (c *Conf) WithHandle(handle func(b []byte)) {
	c.handle = handle
}
