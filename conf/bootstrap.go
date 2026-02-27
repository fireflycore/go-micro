package conf

type BootstrapConf interface {
	GetAppId() string
	GetAppName() string
	GetAppVersion() string
	GetServiceEndpoint() string
	GetServiceAuthToken() string

	GetSystemName() string
	GetSystemType() uint32
	GetSystemVersion() string

	GetGatewayEndpoint() string
}
