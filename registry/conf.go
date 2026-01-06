package registry

import "github.com/fireflycore/go-micro/constant"

// ServiceConf 服务注册/服务发现配置。
type ServiceConf struct {
	// 命名空间
	Namespace string `json:"namespace"`
	// 网卡
	Network *Network `json:"network"`
	// 内核
	Kernel *Kernel `json:"kernel"`

	// 最大重试次数, 间隔时间是TTL*5
	MaxRetry uint32 `json:"max_retry"`
	// 心跳/租约 TTL（秒）, 最少是10s
	TTL uint32 `json:"ttl"`
}

func (sc *ServiceConf) Bootstrap() {
	if sc.Namespace == "" {
		sc.Namespace = constant.DefaultNamespace
	}
	if sc.MaxRetry < 3 {
		sc.MaxRetry = constant.DefaultMaxRetry
	}
	if sc.TTL < 10 {
		sc.TTL = constant.DefaultTTL
	}

	if sc.Kernel == nil {
		sc.Kernel = &Kernel{}
	}
	sc.Kernel.Bootstrap()

	if sc.Network == nil {
		sc.Network = &Network{}
	}
	sc.Network.Bootstrap()
}

// GatewayConf 定义网关相关配置。
type GatewayConf struct {
	// 网卡
	Network *Network `json:"network"`
}

func (gc *GatewayConf) Bootstrap() {
	if gc.Network == nil {
		gc.Network = &Network{}
	}
	gc.Network.Bootstrap()
}
