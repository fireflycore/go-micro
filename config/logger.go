package config

type LoggerConfig interface {
	IsEnableConsole() bool
	IsEnableRemote() bool
}
