package service

type Config struct {
	// Name 表示业务服务名，例如 auth。
	Name string `json:"name"`
	// Type 表示服务类型片段，默认值通常为 svc。
	Type string `json:"type"`
	// Namespace 表示命名空间，例如 default。
	Namespace string `json:"namespace"`
	// ClusterDomain 表示集群域，默认值通常为 cluster.local。
	ClusterDomain string `json:"cluster_domain"`
	// Port 表示业务服务监听端口，默认值通常为 9090。
	Port uint `json:"port"`
	// Weight 表示权重，默认值通常为 100。
	Weight uint `json:"weight"`
}
