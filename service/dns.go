package service

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

// WithPort 返回带显式端口的新 DNS。
func (d DNS) WithPort(port uint16) DNS {
	d.Port = port
	return d
}
