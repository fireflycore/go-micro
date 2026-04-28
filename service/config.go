package service

type Config struct {
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
