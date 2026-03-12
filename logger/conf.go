package logger

// Conf 是 logger 的配置项
// - Console：是否启用控制台输出
// - Remote：是否启用远端输出（OTel）
type Conf struct {
	Console bool `json:"console"`
	Remote  bool `json:"remote"`
}

func (c *Conf) GetLoggerConsole() bool {
	if c == nil {
		return false
	}
	return c.Console
}

func (c *Conf) GetLoggerRemote() bool {
	if c == nil {
		return false
	}
	return c.Remote
}
