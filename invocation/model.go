package invocation

import (
	"fmt"
	"net"
	"strings"
)

const (
	// DefaultClusterDomain 是 K8s Service 的默认集群域。
	// 在标准 K8s + Istio 路径下，最终拨号目标通常会落到该域名体系下。
	DefaultClusterDomain = "svc.cluster.local"
	// DefaultResolverScheme 是 gRPC 推荐使用的 DNS resolver scheme。
	// 统一生成 dns:/// 前缀可以让 target 在不同实现中保持一致。
	DefaultResolverScheme = "dns"
)

// ServiceRef 表示一次调用面向的“服务身份”。
//
// 这里表达的是“我要调用哪个服务”，而不是“我要连接哪个节点”。
// 因此它只保留服务级别的最小身份信息，不承载节点列表等旧 registry 语义。
type ServiceRef struct {
	// Service 表示目标服务名。
	Service string `json:"service"`
	// Namespace 表示目标命名空间。
	// 在 K8s 主路径下，最终 DNS 主要依赖 service + namespace 组合生成。
	Namespace string `json:"namespace"`
	// Env 表示逻辑环境。
	// 它更适合参与平台治理、配置域和策略域选择，而不一定直接进入 DNS。
	Env string `json:"env"`
	// Port 表示可选端口覆盖项。
	// 大多数场景建议由核心库使用默认端口补齐；只有个别服务端口特殊时才显式设置。
	Port uint16 `json:"port"`
}

// Validate 校验 ServiceRef 是否具备生成逻辑目标所需的最小字段。
//
// 当前版本仍然要求 namespace 显式存在，
// 因为这是最稳定、最容易排障的入参形式。
func (s ServiceRef) Validate() error {
	if strings.TrimSpace(s.Service) == "" {
		return ErrServiceNameEmpty
	}
	if strings.TrimSpace(s.Namespace) == "" {
		return ErrNamespaceEmpty
	}
	return nil
}

// ServiceName 返回去除首尾空格后的服务名。
func (s ServiceRef) ServiceName() string {
	return strings.TrimSpace(s.Service)
}

// NamespaceName 返回去除首尾空格后的命名空间。
func (s ServiceRef) NamespaceName() string {
	return strings.TrimSpace(s.Namespace)
}

// EffectivePort 返回最终应使用的端口。
//
// 规则如下：
// 1. 若 ServiceRef 自身显式提供端口，则优先使用；
// 2. 否则尝试使用调用方提供的默认端口；
// 3. 若两者都不可用，则返回错误。
func (s ServiceRef) EffectivePort(defaultPort uint16) (uint16, error) {
	if s.Port != 0 {
		return s.Port, nil
	}
	if defaultPort != 0 {
		return defaultPort, nil
	}
	return 0, ErrTargetPortInvalid
}

// WithPort 返回带端口覆盖项的新 ServiceRef。
// 该方法适合在默认端口不满足时，以不可变方式构造新的服务身份。
func (s ServiceRef) WithPort(port uint16) ServiceRef {
	s.Port = port
	return s
}

// Target 表示最终可拨号的服务目标。
//
// 它是 Locator 解析后的结果：
// - Host/Port 用于实际网络连接；
// - ResolverScheme 用于生成标准的 gRPC target；
// - Address 是 host:port 的网络地址表示。
type Target struct {
	// ResolverScheme 表示 gRPC resolver scheme，例如 dns。
	ResolverScheme string `json:"resolver_scheme"`
	// Host 表示目标主机名。
	Host string `json:"host"`
	// Port 表示目标端口。
	Port uint16 `json:"port"`
}

// Validate 校验 Target 是否具备生成拨号地址所需的字段。
func (t Target) Validate() error {
	if strings.TrimSpace(t.Host) == "" {
		return ErrTargetHostEmpty
	}
	if t.Port == 0 {
		return ErrTargetPortInvalid
	}
	return nil
}

// Address 返回 host:port 形式的网络地址。
func (t Target) Address() string {
	return net.JoinHostPort(strings.TrimSpace(t.Host), fmt.Sprint(t.Port))
}

// GRPCTarget 返回适合 grpc.NewClient 使用的 target 字符串。
//
// 例如：
// - dns:///auth.default.svc.cluster.local:9000
// - 若 ResolverScheme 为空，则退化为普通 host:port。
func (t Target) GRPCTarget() string {
	address := t.Address()
	if strings.TrimSpace(t.ResolverScheme) == "" {
		return address
	}
	return fmt.Sprintf("%s:///%s", t.ResolverScheme, address)
}

// ServiceEndpoint 表示底层实例级端点。
//
// 它只服务于 Locator、Dialer、轻量实现内部缓存等基础设施场景，
// 不再作为业务调用侧的中心模型暴露。
type ServiceEndpoint struct {
	// Address 表示实例地址，通常是 ip:port 或 host:port。
	Address string `json:"address"`
	// Weight 表示实例权重。
	Weight int `json:"weight"`
	// Healthy 表示实例当前健康状态。
	Healthy bool `json:"healthy"`
	// Meta 表示实例附带的轻量元信息，例如标签、版本、可用区等。
	Meta map[string]string `json:"meta"`
}

// TargetOptions 表示将 ServiceRef 解析为 Target 时使用的公共选项。
type TargetOptions struct {
	// DefaultPort 表示默认 gRPC 端口。
	// 当 ServiceRef.Port 为空时，会使用该字段补齐。
	DefaultPort uint16
	// ClusterDomain 表示集群域，默认值为 svc.cluster.local。
	ClusterDomain string
	// ResolverScheme 表示 gRPC resolver scheme，默认值为 dns。
	ResolverScheme string
}

// normalize 补齐 TargetOptions 的默认值。
func (o TargetOptions) normalize() TargetOptions {
	if strings.TrimSpace(o.ClusterDomain) == "" {
		o.ClusterDomain = DefaultClusterDomain
	}
	if strings.TrimSpace(o.ResolverScheme) == "" {
		o.ResolverScheme = DefaultResolverScheme
	}
	return o
}

// BuildTarget 根据 ServiceRef 构造标准 Target。
//
// 该函数是 K8s + Istio 主路径的默认实现形态：
// 通过 service + namespace + clusterDomain 生成统一主机名，
// 再配合端口和 resolver scheme 得到最终 gRPC target。
func BuildTarget(ref ServiceRef, options TargetOptions) (Target, error) {
	if err := ref.Validate(); err != nil {
		return Target{}, err
	}

	options = options.normalize()

	port, err := ref.EffectivePort(options.DefaultPort)
	if err != nil {
		return Target{}, err
	}

	host := fmt.Sprintf("%s.%s.%s", ref.ServiceName(), ref.NamespaceName(), options.ClusterDomain)
	target := Target{
		ResolverScheme: options.ResolverScheme,
		Host:           host,
		Port:           port,
	}
	if err := target.Validate(); err != nil {
		return Target{}, err
	}

	return target, nil
}
