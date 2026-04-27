package service

import "fmt"

const (
	// DefaultNamespace 是业务服务默认使用的命名空间。
	DefaultNamespace = "default"
	// DefaultServiceType 是 Kubernetes Service FQDN 中的固定服务类型片段。
	DefaultServiceType = "svc"
	// DefaultClusterDomain 是 Kubernetes 集群默认的 Cluster Domain。
	DefaultClusterDomain = "cluster.local"
	// DefaultServicePort 是业务服务默认使用的 gRPC 端口。
	DefaultServicePort = 9090
)

// DNS 表示一个业务服务的标准 DNS 配置。
//
// 这里表达的是“这个业务服务在网络上的稳定入口”，
// 而不是“这个服务当前有哪些实例”。
type DNS struct {
	// Service 表示业务服务名，例如 auth。
	Service string `json:"service"`
	// Namespace 表示命名空间，例如 default。
	Namespace string `json:"namespace"`
	// ServiceType 表示服务类型片段，默认值通常为 svc。
	ServiceType string `json:"service_type"`
	// ClusterDomain 表示集群域，默认值通常为 cluster.local。
	ClusterDomain string `json:"cluster_domain"`
	// Port 表示业务服务监听端口，默认值通常为 9090。
	Port uint16 `json:"port"`
}

// Normalize 补齐 DNS 的默认值。
func (d *DNS) Normalize() {
	if d.Namespace == "" {
		d.Namespace = DefaultNamespace
	}
	if d.ServiceType == "" {
		d.ServiceType = DefaultServiceType
	}
	if d.ClusterDomain == "" {
		d.ClusterDomain = DefaultClusterDomain
	}
	if d.Port == 0 {
		d.Port = DefaultServicePort
	}
}

// Build 返回服务的 DNS 名称，例如 demo.default.svc.cluster.local。
func (d *DNS) Build(service string) string {
	return service + "." + d.Namespace + "." + d.ServiceType + "." + d.ClusterDomain
}

// BuildAddress 返回服务的 DNS 地址，例如 demo.default.svc.cluster.local:9090。
func (d *DNS) BuildAddress(service string) string {
	return fmt.Sprintf("%s:%d", d.Build(service), d.Port)
}
