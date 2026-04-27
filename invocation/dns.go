package invocation

import (
	"fmt"
	"strings"

	svc "github.com/fireflycore/go-micro/service"
)

const (
	// DefaultResolverScheme 是 gRPC 默认推荐使用的 DNS resolver scheme。
	DefaultResolverScheme = "dns"
	// DefaultNamespace 是业务服务默认使用的命名空间。
	DefaultNamespace = "default"
	// DefaultServiceType 是 Kubernetes Service FQDN 中的固定服务类型片段。
	DefaultServiceType = "svc"
	// DefaultClusterDomain 是 Kubernetes 集群默认的 Cluster Domain。
	DefaultClusterDomain = "cluster.local"
	// DefaultServicePort 是业务服务默认使用的 gRPC 端口。
	DefaultServicePort = 9090
)

// validateDNS 校验 service.DNS 是否具备构造最终目标的最小字段集。
func validateDNS(dns *svc.DNS) error {
	// DNS 结构本身不能为空。
	if dns == nil || strings.TrimSpace(dns.Service) == "" {
		// 服务名为空时直接返回错误，避免后续拼接出无效 host。
		return ErrServiceNameEmpty
	}
	// 命名空间也是 FQDN 的必要组成部分。
	if strings.TrimSpace(dns.Namespace) == "" {
		// 命名空间为空时同样不能形成有效地址。
		return ErrNamespaceEmpty
	}
	// 校验通过时返回 nil。
	return nil
}

// effectivePort 解析最终用于拨号的端口。
func effectivePort(dns *svc.DNS, defaultPort uint16) (uint16, error) {
	// 如果业务侧显式指定了端口，则优先使用显式端口。
	if dns != nil && dns.Port != 0 {
		// 显式端口优先级最高。
		return dns.Port, nil
	}
	// 否则使用平台默认端口。
	if defaultPort != 0 {
		// 默认端口可用时直接返回默认值。
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
		c.DefaultNamespace = DefaultNamespace
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

// defaultDNSConfig 保存一份进程级默认配置，避免 nil 配置场景重复归一化。
var defaultDNSConfig = DNSConfig{}.normalize()

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
		// 直接复用进程级默认配置指针，减少重复分配。
		return &DNSManager{config: defaultDNSConfig}
	}
	// 非 nil 配置仍然在创建时完成一次归一化。
	return &DNSManager{
		config: config.normalize(),
	}
}

// configOrDefault 返回当前管理器可用的配置指针。
func (m *DNSManager) configOrDefault() *DNSConfig {
	// nil 管理器或 nil 配置都退化到默认配置。
	if m == nil || m.config == nil {
		return defaultDNSConfig
	}
	// 否则直接返回内部配置指针，避免额外复制。
	return m.config
}

// Config 返回当前管理器的规范化配置副本。
func (m *DNSManager) Config() *DNSConfig {
	// 若调用方传入的是 nil，则返回默认配置的副本。
	config := m.configOrDefault()
	// 返回副本而不是内部指针，避免调用方误改管理器内部配置。
	cfg := *config
	// 返回副本给外部读取。
	return &cfg
}

// Normalize 用默认配置补齐业务服务 DNS。
func (m *DNSManager) Normalize(dns *svc.DNS) *svc.DNS {
	// 若上游传入 nil，则在本地构造一份空 DNS 再补默认值。
	if dns == nil {
		// 创建一份新的空结构，后续统一在原对象上补值。
		dns = &svc.DNS{}
	}
	// 先拿到一份可用的默认配置。
	config := m.configOrDefault()
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
	// 返回被补齐后的同一个指针，避免二次复制。
	return dns
}

// Build 根据 service.DNS 构造最终 Target。
func (m *DNSManager) Build(dns *svc.DNS) (*Target, error) {
	// 先拿到可直接复用的配置指针，避免多次生成副本。
	config := m.configOrDefault()
	// 先补齐默认值。
	dns = m.Normalize(dns)
	// 再校验最小必要字段。
	if err := validateDNS(dns); err != nil {
		return &Target{}, err
	}
	port, err := effectivePort(dns, config.DefaultPort)
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
		// ResolverScheme 使用归一化后的统一配置。
		ResolverScheme: config.ResolverScheme,
		// Host 使用标准 Kubernetes Service FQDN。
		Host: host,
		// Port 使用显式端口或默认端口解析结果。
		Port: port,
	}
	// 预先缓存派生字符串，避免后续热路径重复拼接。
	target.cacheDerivedStrings()
	// 最终目标再做一次校验。
	if err := target.Validate(); err != nil {
		return &Target{}, err
	}

	// 返回构造完成的可拨号目标。
	return &target, nil
}
