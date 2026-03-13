package conf

type LoggerConf interface {
	GetLoggerConsole() bool
	GetLoggerRemote() bool
}
