package invocation

import (
	"fmt"
	"strings"

	svc "github.com/fireflycore/go-micro/service"
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

func validateDNS(dns svc.DNS) error {
	// 服务名是唯一必须由业务侧显式提供的字段。
	if strings.TrimSpace(dns.Service) == "" {
		return ErrServiceNameEmpty
	}
	// 命名空间在生成完整 FQDN 时也是必需的。
	if strings.TrimSpace(dns.Namespace) == "" {
		return ErrNamespaceEmpty
	}
	return nil
}

func effectivePort(dns svc.DNS, defaultPort uint16) (uint16, error) {
	// 如果业务侧显式指定了端口，则优先使用显式端口。
	if dns.Port != 0 {
		return dns.Port, nil
	}
	// 否则使用平台默认端口。
	if defaultPort != 0 {
		return defaultPort, nil
	}
	// 两者都没有时，无法形成可拨号目标。
	return 0, ErrTargetPortInvalid
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

// DNSManager 负责把结构化的 service.DNS 转成最终 Target。
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
func (m *DNSManager) Normalize(dns *svc.DNS) *svc.DNS {
	// 若上游传入 nil，则在本地构造一份空 DNS 再补默认值。
	if dns == nil {
		dns = &svc.DNS{}
	}
	// 先拿到一份可用的默认配置。
	config := m.Config()
	// 若业务侧未填写 namespace，则使用默认 namespace。
	if strings.TrimSpace(dns.Namespace) == "" {
		dns.Namespace = config.DefaultNamespace
	}
	// 若业务侧未填写 service type，则使用默认 svc。
	if strings.TrimSpace(dns.ServiceType) == "" {
		dns.ServiceType = config.DefaultServiceType
	}
	// 若业务侧未填写 cluster domain，则使用默认 cluster.local。
	if strings.TrimSpace(dns.ClusterDomain) == "" {
		dns.ClusterDomain = config.DefaultClusterDomain
	}
	// 若业务侧未填写端口，则使用默认端口。
	if dns.Port == 0 {
		dns.Port = config.DefaultPort
	}
	return dns
}

// Build 根据 service.DNS 构造最终 Target。
func (m *DNSManager) Build(dns *svc.DNS) (*Target, error) {
	// 先补齐默认值。
	dns = m.Normalize(dns)
	// 再校验最小必要字段。
	if err := validateDNS(*dns); err != nil {
		return &Target{}, err
	}
	port, err := effectivePort(*dns, m.Config().DefaultPort)
	if err != nil {
		return &Target{}, err
	}
	// 拼出标准主机名：service.namespace.svc.cluster.local。
	host := fmt.Sprintf(
		"%s.%s.%s.%s",
		strings.TrimSpace(dns.Service),
		strings.TrimSpace(dns.Namespace),
		strings.TrimSpace(dns.ServiceType),
		strings.TrimSpace(dns.ClusterDomain),
	)
	// 组装最终 Target。
	target := Target{
		ResolverScheme: m.Config().ResolverScheme,
		Host:           host,
		Port:           port,
	}
	// 最终目标再做一次校验。
	if err := target.Validate(); err != nil {
		return &Target{}, err
	}

	return &target, nil
}
