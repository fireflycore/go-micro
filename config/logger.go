package config

type LoggerConfig interface {
	GetLoggerConsole() bool
	GetLoggerRemote() bool
}
