package kubernetes

import (
	"fmt"
	"strings"
)

// Discovery 基于 K8S DNS 的服务发现实现。
type Discovery struct {
	defaultNamespace string
	defaultPort      int
	// portMap 存储特殊服务的端口映射 (appId -> port)
	portMap map[string]int
}

// DiscoveryOption 定义 Discovery 的可选配置项。
type DiscoveryOption func(*Discovery)

// WithPortMap 设置特殊服务的端口映射。
// 适用于无法统一端口的场景，例如:
//
//	WithPortMap(map[string]int{
//	    "redis": 6379,
//	    "mysql": 3306,
//	})
func WithPortMap(m map[string]int) DiscoveryOption {
	return func(d *Discovery) {
		d.portMap = m
	}
}

// NewDiscovery 创建一个基于 DNS 的发现器。
// namespace: 默认命名空间（通常为 "default" 或当前 Pod 所在 NS）。
// port: 默认 gRPC 服务端口（通常团队内会有统一规范，如 9090）。
// opts: 可选配置，如特殊端口映射。
func NewDiscovery(namespace string, port int, opts ...DiscoveryOption) *Discovery {
	d := &Discovery{
		defaultNamespace: namespace,
		defaultPort:      port,
		portMap:          make(map[string]int),
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// GetTarget 返回 gRPC Dial 可用的 Target 字符串。
// 格式：dns:///{service_name}.{namespace}:{port}
//
// 端口解析优先级：
// 1. appId 中显式指定 (如 "user:8080")
// 2. PortMap 中配置的特殊映射 (如 "redis" -> 6379)
// 3. 默认端口 (defaultPort)
func (d *Discovery) GetTarget(appId string) string {
	nameAndNs := appId
	var port int

	// 1. 尝试从 appId 中解析端口
	if idx := strings.LastIndex(appId, ":"); idx != -1 {
		// 如果 appId 自带端口，直接使用
		// 简单的端口提取，暂不处理 IPv6 等复杂情况
		p := appId[idx+1:]
		nameAndNs = appId[:idx]
		return fmt.Sprintf("dns:///%s:%s", d.ensureNamespace(nameAndNs), p)
	}

	// 2. 尝试从映射表中查找端口
	if p, ok := d.portMap[appId]; ok {
		port = p
	} else {
		// 3. 使用默认端口
		port = d.defaultPort
	}

	return fmt.Sprintf("dns:///%s:%d", d.ensureNamespace(nameAndNs), port)
}

// ensureNamespace 确保服务名包含命名空间
func (d *Discovery) ensureNamespace(name string) string {
	if strings.Contains(name, ".") {
		// 假设已经包含了命名空间 (如 user.default)
		return name
	}
	if d.defaultNamespace == "" {
		return name
	}
	return fmt.Sprintf("%s.%s", name, d.defaultNamespace)
}
