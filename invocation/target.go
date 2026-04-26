package invocation

import (
	"fmt"
	"net"
	"strings"
)

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

	// address 缓存标准 host:port，避免重复 JoinHostPort。
	address string
	// grpcTarget 缓存最终 gRPC target，避免重复格式化字符串。
	grpcTarget string
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
	// 若已经预计算过 address，则直接复用缓存。
	if t.address != "" {
		return t.address
	}
	// 否则按 host:port 即时拼装。
	return net.JoinHostPort(strings.TrimSpace(t.Host), fmt.Sprint(t.Port))
}

// GRPCTarget 返回适合 grpc.NewClient 使用的 target 字符串。
func (t Target) GRPCTarget() string {
	// 若已经缓存过最终 target，则直接返回缓存值。
	if t.grpcTarget != "" {
		return t.grpcTarget
	}
	// 先得到标准 address。
	address := t.Address()
	// 如果没有 resolver scheme，则退化为普通 host:port。
	if strings.TrimSpace(t.ResolverScheme) == "" {
		// 未配置 resolver scheme 时退化为普通 address。
		return address
	}
	// 否则返回 gRPC 推荐的 dns:/// 前缀形式。
	return fmt.Sprintf("%s:///%s", t.ResolverScheme, address)
}

// cacheDerivedStrings 在目标创建后一次性缓存派生字符串。
func (t *Target) cacheDerivedStrings() {
	// nil 指针直接返回，避免空指针解引用。
	if t == nil {
		return
	}
	// 先缓存标准 host:port。
	t.address = net.JoinHostPort(strings.TrimSpace(t.Host), fmt.Sprint(t.Port))
	if strings.TrimSpace(t.ResolverScheme) == "" {
		// 无 resolver scheme 时最终 target 就是 address 本身。
		t.grpcTarget = t.address
		return
	}
	// 有 resolver scheme 时缓存带 scheme 的 gRPC target。
	t.grpcTarget = fmt.Sprintf("%s:///%s", t.ResolverScheme, t.address)
}
