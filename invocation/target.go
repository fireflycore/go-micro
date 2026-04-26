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
