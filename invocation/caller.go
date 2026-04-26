package invocation

import (
	"context"

	svc "github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc"
)

// RemoteServiceCaller 表示一个远程业务服务的通用调用入口。
//
// 这个对象绑定的是“远程业务服务”而不是“某个 proto 子服务”。
// 因此一个业务服务下多个 proto 子服务，应共用同一个 RemoteServiceCaller。
//
// 例如：
// - auth 业务服务
//   - AuthAppService
//   - AuthUserService
//   - AuthPermissionService
//
// 这些子服务都应共用一份：
// - service.DNS
// - ConnectionManager
// - UnaryInvoker
type RemoteServiceCaller struct {
	// Service 表示当前远程业务服务的标准 DNS 身份。
	Service *svc.DNS
	// Invoker 负责统一的连接获取、metadata 注入和实际调用。
	//
	// 当前约束下 RemoteServiceCaller 只是一个薄封装，
	// 真正的调用行为仍全部落在 UnaryInvoker 上。
	Invoker *UnaryInvoker
}

// NewRemoteServiceCaller 创建一个标准的远程业务服务调用器。
//
// 这个构造函数的目标是把业务侧最常见的装配模板统一收口：
// - 指定远程业务服务 DNS；
// - 指定统一复用的 UnaryInvoker。
func NewRemoteServiceCaller(invoker *UnaryInvoker, dns *svc.DNS) *RemoteServiceCaller {
	return &RemoteServiceCaller{
		Service: dns,
		Invoker: invoker,
	}
}

// Invoke 对当前绑定的远程业务服务发起一次 unary 调用。
//
// 调用方只需要提供：
// - full method
// - request
// - response
//
// 其余通用逻辑由 service.DNS 与 UnaryInvoker 统一处理。
func (c *RemoteServiceCaller) Invoke(ctx context.Context, method string, req any, resp any, callOptions ...grpc.CallOption) error {
	// 若没有绑定 Invoker，则无法发起调用。
	if c == nil || c.Invoker == nil {
		return ErrInvokerDialerIsNil
	}

	// 最终仍由 UnaryInvoker 统一完成真正的 gRPC 调用。
	return c.Invoker.Invoke(ctx, c.Service, method, req, resp, callOptions...)
}
