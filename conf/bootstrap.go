package conf

type BootstrapConf interface {
	GetAppId() string
	GetServiceEndpoint() string
	GetServiceAuthToken() string

	GetClientName() string
	GetClientType() uint32
	GetClientVersion() string

	GetSystemName() string
	GetSystemType() uint32
	GetSystemVersion() string
}
