package micro

// ServiceConf 服务注册/服务发现配置。
type ServiceConf struct {
	// 命名空间
	Namespace string `json:"namespace"`
	// 网卡
	Network *Network `json:"network"`
	// 内核
	Kernel *Kernel `json:"kernel"`

	// 最大重试次数
	MaxRetry uint32 `json:"max_retry"`
	// 心跳间隔
	TTL uint32 `json:"ttl"`
}

// GatewayConf 定义网关相关配置。
type GatewayConf struct {
	// 网卡
	Network Network `json:"network"`
}
