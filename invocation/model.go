package invocation

import (
	"fmt"
	"net"
	"strings"
)

const (
	// DefaultServiceType 是 Kubernetes Service FQDN 中的固定服务类型片段。
	DefaultServiceType = "svc"
	// DefaultClusterDomain 是 Kubernetes 集群默认的 Cluster Domain。
	DefaultClusterDomain = "cluster.local"
	// DefaultResolverScheme 是 gRPC 默认推荐使用的 DNS resolver scheme。
	DefaultResolverScheme = "dns"
	// DefaultServicePort 是业务服务默认使用的 gRPC 端口。
	DefaultServicePort = 9090
)

// ServiceDNS 表示一个业务服务的标准 DNS 配置。
//
// 这里表达的是“这个业务服务在网络上的稳定入口”，
// 而不是“这个服务当前有哪些实例”。
//
// 例如：
// - service: auth
// - namespace: default
// - service_type: svc
// - cluster_domain: cluster.local
// - port: 9090
//
// 最终会得到：
// - host: auth.default.svc.cluster.local
// - address: auth.default.svc.cluster.local:9090
type ServiceDNS struct {
	// Service 表示业务服务名，例如 auth。
	Service string `json:"service"`
	// Namespace 表示命名空间，例如 default。
	Namespace string `json:"namespace"`
	// ServiceType 表示服务类型片段，默认值为 svc。
	ServiceType string `json:"service_type"`
	// ClusterDomain 表示集群域，默认值为 cluster.local。
	ClusterDomain string `json:"cluster_domain"`
	// Port 表示业务服务监听端口，默认值为 9090。
	Port uint16 `json:"port"`
}

// Validate 检查 ServiceDNS 是否具备生成稳定 DNS 的最小字段。
func (s ServiceDNS) Validate() error {
	// 服务名是唯一必须由业务侧显式提供的字段。
	if strings.TrimSpace(s.Service) == "" {
		return ErrServiceNameEmpty
	}
	// 命名空间在生成完整 FQDN 时也是必需的。
	if strings.TrimSpace(s.Namespace) == "" {
		return ErrNamespaceEmpty
	}
	return nil
}

// ServiceName 返回清理空格后的服务名。
func (s ServiceDNS) ServiceName() string {
	return strings.TrimSpace(s.Service)
}

// NamespaceName 返回清理空格后的命名空间。
func (s ServiceDNS) NamespaceName() string {
	return strings.TrimSpace(s.Namespace)
}

// ServiceTypeName 返回清理空格后的服务类型。
func (s ServiceDNS) ServiceTypeName() string {
	return strings.TrimSpace(s.ServiceType)
}

// ClusterDomainName 返回清理空格后的集群域名。
func (s ServiceDNS) ClusterDomainName() string {
	return strings.TrimSpace(s.ClusterDomain)
}

// EffectivePort 返回当前 ServiceDNS 的最终端口。
func (s ServiceDNS) EffectivePort(defaultPort uint16) (uint16, error) {
	// 如果业务侧显式指定了端口，则优先使用显式端口。
	if s.Port != 0 {
		return s.Port, nil
	}
	// 否则使用平台默认端口。
	if defaultPort != 0 {
		return defaultPort, nil
	}
	// 两者都没有时，无法形成可拨号目标。
	return 0, ErrTargetPortInvalid
}

// WithPort 返回带显式端口的新 ServiceDNS。
func (s ServiceDNS) WithPort(port uint16) ServiceDNS {
	s.Port = port
	return s
}

// Target 表示最终可拨号的 gRPC 服务目标。
//
// 它已经不表达实例列表，只表达一个稳定的服务入口。
type Target struct {
	// ResolverScheme 表示 gRPC resolver scheme，例如 dns。
	ResolverScheme string `json:"resolver_scheme"`
	// Host 表示服务的标准主机名。
	Host string `json:"host"`
	// Port 表示服务端口。
	Port uint16 `json:"port"`
}

// Validate 检查 Target 是否可以用于真实拨号。
func (t Target) Validate() error {
	// 主机名不能为空。
	if strings.TrimSpace(t.Host) == "" {
		return ErrTargetHostEmpty
	}
	// 端口必须大于 0。
	if t.Port == 0 {
		return ErrTargetPortInvalid
	}
	return nil
}

// Address 返回标准的 host:port 地址。
func (t Target) Address() string {
	return net.JoinHostPort(strings.TrimSpace(t.Host), fmt.Sprint(t.Port))
}

// GRPCTarget 返回适合 grpc.NewClient 使用的 target 字符串。
func (t Target) GRPCTarget() string {
	// 先得到标准 address。
	address := t.Address()
	// 如果没有 resolver scheme，则退化为普通 host:port。
	if strings.TrimSpace(t.ResolverScheme) == "" {
		return address
	}
	// 否则返回 gRPC 推荐的 dns:/// 前缀形式。
	return fmt.Sprintf("%s:///%s", t.ResolverScheme, address)
}

// DNSConfig 定义标准 DNS 管理器的默认行为。
type DNSConfig struct {
	// DefaultNamespace 表示默认命名空间。
	DefaultNamespace string
	// DefaultServiceType 表示默认服务类型片段。
	DefaultServiceType string
	// DefaultClusterDomain 表示默认集群域名。
	DefaultClusterDomain string
	// DefaultPort 表示默认端口。
	DefaultPort uint16
	// ResolverScheme 表示默认 gRPC resolver scheme。
	ResolverScheme string
}

// normalize 补齐 DNSConfig 的默认值。
func (c DNSConfig) normalize() *DNSConfig {
	// 默认命名空间统一用 default。
	if strings.TrimSpace(c.DefaultNamespace) == "" {
		c.DefaultNamespace = "default"
	}
	// 默认服务类型统一用 svc。
	if strings.TrimSpace(c.DefaultServiceType) == "" {
		c.DefaultServiceType = DefaultServiceType
	}
	// 默认集群域统一用 cluster.local。
	if strings.TrimSpace(c.DefaultClusterDomain) == "" {
		c.DefaultClusterDomain = DefaultClusterDomain
	}
	// 默认端口统一用 9090。
	if c.DefaultPort == 0 {
		c.DefaultPort = DefaultServicePort
	}
	// 默认 resolver 统一用 dns。
	if strings.TrimSpace(c.ResolverScheme) == "" {
		c.ResolverScheme = DefaultResolverScheme
	}
	return &c
}

// DNSManager 负责把结构化的 ServiceDNS 转成最终 Target。
//
// 它只做一件事：组装标准 DNS。
// 它不做实例发现、不做节点选择，也不做后端适配。
type DNSManager struct {
	config *DNSConfig
}

// NewDNSManager 创建一个标准 DNS 管理器。
func NewDNSManager(config *DNSConfig) *DNSManager {
	// 若调用方没有传配置，则使用一份空配置走默认值补齐逻辑。
	if config == nil {
		config = &DNSConfig{}
	}
	return &DNSManager{
		config: config.normalize(),
	}
}

// Config 返回当前管理器的规范化配置副本。
func (m *DNSManager) Config() *DNSConfig {
	// 若调用方传入的是 nil，则返回默认配置的副本。
	if m == nil {
		return DNSConfig{}.normalize()
	}
	// 返回副本而不是内部指针，避免调用方误改管理器内部配置。
	cfg := *m.config
	return &cfg
}

// Normalize 用默认配置补齐业务服务 DNS。
func (m *DNSManager) Normalize(service *ServiceDNS) *ServiceDNS {
	// 若上游传入 nil，则在本地构造一份空 ServiceDNS 再补默认值。
	if service == nil {
		service = &ServiceDNS{}
	}
	// 先拿到一份可用的默认配置。
	config := m.Config()
	// 若业务侧未填写 namespace，则使用默认 namespace。
	if strings.TrimSpace(service.Namespace) == "" {
		service.Namespace = config.DefaultNamespace
	}
	// 若业务侧未填写 service type，则使用默认 svc。
	if strings.TrimSpace(service.ServiceType) == "" {
		service.ServiceType = config.DefaultServiceType
	}
	// 若业务侧未填写 cluster domain，则使用默认 cluster.local。
	if strings.TrimSpace(service.ClusterDomain) == "" {
		service.ClusterDomain = config.DefaultClusterDomain
	}
	// 若业务侧未填写端口，则使用默认端口。
	if service.Port == 0 {
		service.Port = config.DefaultPort
	}
	return service
}

// Build 根据 ServiceDNS 构造最终 Target。
func (m *DNSManager) Build(service *ServiceDNS) (*Target, error) {
	// 先补齐默认值。
	service = m.Normalize(service)
	// 再校验最小必要字段。
	if err := service.Validate(); err != nil {
		return &Target{}, err
	}
	// 拼出标准主机名：service.namespace.svc.cluster.local。
	host := fmt.Sprintf(
		"%s.%s.%s.%s",
		service.ServiceName(),
		service.NamespaceName(),
		service.ServiceTypeName(),
		service.ClusterDomainName(),
	)
	// 组装最终 Target。
	target := Target{
		ResolverScheme: m.Config().ResolverScheme,
		Host:           host,
		Port:           service.Port,
	}
	// 最终目标再做一次校验。
	if err := target.Validate(); err != nil {
		return &Target{}, err
	}

	return &target, nil
}

// BuildTarget 是兼容保留的辅助函数。
//
// 新代码建议直接使用 DNSManager.Build。
func BuildTarget(service *ServiceDNS, config *DNSConfig) (*Target, error) {
	return NewDNSManager(config).Build(service)
}
