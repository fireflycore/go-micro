package config

type BootstrapConfig interface {
	GetAppId() string
	GetAppSecret() string
	GetAppName() string
	GetAppVersion() string
	GetServiceEndpoint() string
	GetServiceNamespace() string
	GetServiceInstanceId() string

	GetSystemName() string
	GetSystemType() uint32
	GetSystemVersion() string

	GetServerPort() uint
	GetManagementPort() uint

	LoggerConfig
	TelemetryConfig
}
