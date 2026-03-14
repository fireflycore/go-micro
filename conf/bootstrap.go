package conf

type BootstrapConf interface {
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

	// GetServerPort 获取业务服务端口
	GetServerPort() uint
	// GetManagementPort 获取管理/监控端口
	GetManagementPort() uint

	LoggerConf
	TelemetryConf
}
