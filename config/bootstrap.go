package config

type BootstrapConfig interface {
	GetAppId() string
	GetAppSecret() string
	GetAppName() string
	GetAppVersion() string
	GetServiceEndpoint() string
	GetServiceAuthToken() string
	GetServiceNamespace() string
	GetServiceInstanceId() string

	GetSystemName() string
	GetSystemType() uint32
	GetSystemVersion() string

	GetGatewayEndpoint() string
	GetGatewayAuthToken() string

	GetServerPort() uint
	GetManagementPort() uint

	LoggerConfig
	TelemetryConfig
}
