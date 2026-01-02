package micro

const (
	// Info 表示信息级别日志。
	Info LogLevel = "info"
	// Error 表示错误级别日志。
	Error LogLevel = "error"
	// Success 表示成功级别日志。
	Success LogLevel = "success"
)

// LogLevel 表示日志级别。
type LogLevel string
