package invocation

import (
	"context"

	"google.golang.org/grpc"
)

// Dialer 定义“如何把业务服务 DNS 变成 grpc.ClientConn”。
//
// 在新模型中，Dialer 只关心：
// - 标准 DNS target 组装结果；
// - 连接复用；
// - gRPC 连接创建。
type Dialer interface {
	Dial(ctx context.Context, service *ServiceDNS) (*grpc.ClientConn, error)
	Close() error
}

// Authorizer 定义外挂 Authz 的最小能力集合。
//
// 返回 nil 表示允许调用；
// 返回非 nil error 表示拒绝调用或 Authz 本身发生错误。
type Authorizer interface {
	Authorize(ctx context.Context, input *AuthzContext) error
}

// Invoker 定义统一调用入口。
//
// 与业务代码直接操作 grpc.ClientConn 不同，
// Invoker 允许框架在调用前统一完成：
// - 业务服务 DNS 目标解析；
// - metadata 注入；
// - Authz 预检查；
// - 底层连接复用。
type Invoker interface {
	Invoke(ctx context.Context, service *ServiceDNS, method string, req any, resp any, options ...InvokeOption) error
}
