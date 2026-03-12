package logger

// Conf 是 logger 的配置项
// - Console：是否启用控制台输出
// - Remote：是否启用远端输出（OTel）
type Conf struct {
	Console bool `json:"console"`
	Remote  bool `json:"remote"`
}
