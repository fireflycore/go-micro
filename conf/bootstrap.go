package conf

type BootstrapConf interface {
	GetAppId() string
	GetServiceEndpoint() string
	GetServiceAuthToken() string
}
