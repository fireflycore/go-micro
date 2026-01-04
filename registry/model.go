package registry

import (
	"net"
	"strings"
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

// Network 定义服务节点上报的网络信息。
type Network struct {
	// 网卡唯一标识，用于grpc-gateway流量控制，同sn流量优先
	SN string `json:"sn"`
	// 内网地址
	Internal string `json:"internal"`
	// 外网地址
	External string `json:"external"`
}

// GetInternalNetworkIp 获取本机对外优选 IP（用于内网地址上报）。
func GetInternalNetworkIp() string {
	// 通过建立 UDP “伪连接”获取本机路由选路后的本地地址，不会真正发送业务数据。
	dial, err := net.Dial("udp", "114.114.114.114:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer dial.Close()
	addr := dial.LocalAddr().String()

	index := strings.LastIndex(addr, ":")
	return addr[:index]
}

// ServiceNode 适用于服务注册/发现的节点描述。
type ServiceNode struct {
	ProtoCount int             `json:"proto_count"`
	LeaseId    int             `json:"lease_id"`
	RunDate    string          `json:"run_date"`
	Methods    map[string]bool `json:"methods"`

	Network *Network `json:"network"`
	Kernel  *Kernel  `json:"kernel"`
	Meta    *Meta    `json:"meta"`
}

// ParseMethod 将节点方法映射写入方法表（method -> appId）。
func (ist *ServiceNode) ParseMethod(s ServiceMethods) {
	if ist.Meta == nil || ist.Meta.AppId == "" {
		return
	}
	for k := range ist.Methods {
		s[k] = ist.Meta.AppId
	}
}

// CheckMethod 检查节点是否包含指定方法。
func (ist *ServiceNode) CheckMethod(sm string) error {
	if _, ok := ist.Methods[sm]; ok {
		return nil
	}
	return ErrServiceNodeMethodNotExists
}
