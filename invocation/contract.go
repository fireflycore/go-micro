package invocation

import (
	"context"

	"google.golang.org/grpc"
)

// Locator 定义“如何把服务身份解析为可拨号目标”。
//
// 标准实现下，Locator 可能只是把 service + namespace 组装成 DNS；
// 轻量实现下，Locator 也可以在内部维护 watch、缓存、CoreDNS 同步等复杂逻辑。
// 这些差异都不应泄漏给业务调用方。
type Locator interface {
	Resolve(ctx context.Context, ref ServiceRef) (Target, error)
}

// Dialer 定义“如何把服务身份变成 grpc.ClientConn”。
//
// 这一层通常会组合 Locator 与连接缓存，
// 从而让业务侧始终面向 ServiceRef 而不是 host:port 字符串。
type Dialer interface {
	Dial(ctx context.Context, ref ServiceRef) (*grpc.ClientConn, error)
	Close() error
}

// Authorizer 定义外挂 Authz 的最小能力集合。
//
// 返回 nil 表示允许调用；
// 返回非 nil error 表示拒绝调用或 Authz 本身发生错误。
type Authorizer interface {
	Authorize(ctx context.Context, input AuthzContext) error
}

// Invoker 定义统一调用入口。
//
// 与业务代码直接操作 grpc.ClientConn 不同，
// Invoker 允许框架在调用前统一完成：
// - 服务目标解析；
// - metadata 注入；
// - Authz 预检查；
// - 底层连接复用。
type Invoker interface {
	Invoke(ctx context.Context, ref ServiceRef, method string, req any, resp any, options ...InvokeOption) error
}
