package registry

import (
	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-utils/network"
)

// Meta 服务元信息。
type Meta struct {
	// 环境，不同环境的服务不互通
	Env string `json:"env"`
	// 应用id，泛指服务实例，不同版本的服务实例可以共享appId
	AppId string `json:"app_id"`
	// 服务实例版本
	Version string `json:"version"`
}

// Kernel 定义服务实例运行时元信息。
type Kernel struct {
	// 所使用的开发语言
	Language string `json:"language"`
	// 内核版本
	Version string `json:"version"`
}

func (k *Kernel) Bootstrap() {
	k.Language = constant.KernelLanguage
	if k.Version == "" {
		k.Version = constant.DefaultVersion
	}
}

// Network 定义服务节点上报的网络信息。
type Network struct {
	// 网卡唯一标识，用于grpc-gateway流量控制，同sn流量优先
	SN string `json:"sn"`
	// 内网地址
	Internal string `json:"internal"`
	// 外网地址
	External string `json:"external"`
}

func (n *Network) Bootstrap() {
	if n.SN == "" {
		n.SN = constant.DefaultNetworkSN
	}
	if n.Internal == "" {
		n.Internal = network.GetInternalNetworkIp()
	}
	if n.External == "" {
		n.Internal = constant.DefaultExternalNetworkAddress
	}
}
