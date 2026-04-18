package invocation

import (
	"context"

	"google.golang.org/grpc/metadata"
)

// ContextBuilder 定义“如何为一次远程业务服务调用构造 InvocationContext”。
//
// 之所以把它抽成函数，是为了让业务侧可以把：
// - metadata 透传
// - caller 信息提取
// - timeout 补齐
//
// 收敛到一个统一入口，而不是每个 repo 方法里重复写一遍。
type ContextBuilder func(context.Context) *InvocationContext

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
// - ServiceDNS
// - ConnectionManager
// - UnaryInvoker
type RemoteServiceCaller struct {
	// Service 表示当前远程业务服务的标准 DNS 身份。
	Service *ServiceDNS
	// Invoker 负责统一的连接获取、metadata 注入、Authz 预检查和实际调用。
	Invoker *UnaryInvoker
	// BuildContext 用于把业务侧 context 收敛成标准 InvocationContext。
	// 若为空，则默认使用零值 InvocationContext。
	BuildContext ContextBuilder
}

// NewRemoteServiceCaller 创建一个标准的远程业务服务调用器。
//
// 这个构造函数的目标是把业务侧最常见的装配模板统一收口：
// - 指定远程业务服务 DNS；
// - 指定统一复用的 UnaryInvoker；
// - 指定默认的 InvocationContext 构造逻辑。
func NewRemoteServiceCaller(invoker *UnaryInvoker, service *ServiceDNS, buildContext ContextBuilder) *RemoteServiceCaller {
	return &RemoteServiceCaller{
		Service:      service,
		Invoker:      invoker,
		BuildContext: buildContext,
	}
}

// BuildInvocationContextFromContext 从当前 context 构造一份标准 InvocationContext。
//
// 这是当前推荐的默认上下文构造逻辑：
// - 透传 incoming/outgoing metadata；
// - 透传已解析好的用户上下文；
// - 为后续 trace / request-id 收口保留统一入口。
//
// 若业务侧没有特殊定制需求，可以直接把该函数作为 RemoteServiceCaller 的 BuildContext。
func BuildInvocationContextFromContext(ctx context.Context) *InvocationContext {
	// 先创建一份空的调用上下文。
	out := &InvocationContext{
		Caller: Caller{},
	}

	// 优先透传已有 metadata，保证 trace / auth / caller 语义不断链。
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		out.Metadata = md.Copy()
	} else if md, ok := metadata.FromOutgoingContext(ctx); ok {
		out.Metadata = md.Copy()
	}

	// 没有 metadata 时补一个空壳，避免后续操作出现 nil。
	if out.Metadata == nil {
		out.Metadata = metadata.New(nil)
	}

	// 若上游已经把用户上下文解析到 context，这里直接复用。
	if userMeta, ok := UserContextFromContext(ctx); ok && userMeta != nil {
		out.Caller.UserId = userMeta.UserId
		out.Caller.AppId = userMeta.AppId
		out.Caller.TenantId = userMeta.TenantId
		out.Caller.RoleIds = append([]string(nil), userMeta.RoleIds...)
		out.Caller.OrgIds = append([]string(nil), userMeta.OrgIds...)
	}

	return out
}

// Invoke 对当前绑定的远程业务服务发起一次 unary 调用。
//
// 调用方只需要提供：
// - full method
// - request
// - response
//
// 其余通用逻辑由：
// - ServiceDNS
// - UnaryInvoker
// - ContextBuilder
//
// 统一处理。
func (c *RemoteServiceCaller) Invoke(ctx context.Context, method string, req any, resp any, options ...InvokeOption) error {
	// 若没有绑定 Invoker，则无法发起调用。
	if c == nil || c.Invoker == nil {
		return ErrInvokerDialerIsNil
	}

	// 先复制一份调用选项，避免覆盖调用方显式传入的配置。
	finalOptions := append([]InvokeOption(nil), options...)

	// 只有当调用方没有显式传 InvocationContext 时，才由 BuildContext 补默认上下文。
	if !hasInvocationContextOption(options) {
		invocation := &InvocationContext{}
		if c.BuildContext != nil {
			invocation = c.BuildContext(ctx)
			if invocation == nil {
				invocation = &InvocationContext{}
			}
		}
		finalOptions = append(finalOptions, WithInvocationContext(invocation))
	}

	// 最终仍由 UnaryInvoker 统一完成真正的 gRPC 调用。
	return c.Invoker.Invoke(ctx, c.Service, method, req, resp, finalOptions...)
}

// hasInvocationContextOption 判断调用方是否已经显式设置 InvocationContext。
func hasInvocationContextOption(options []InvokeOption) bool {
	// 用一份零值 InvokeOptions 试跑一遍调用选项。
	probe := InvokeOptions{}
	for _, apply := range options {
		if apply == nil {
			continue
		}
		apply(&probe)
	}
	// 只要 InvocationContext 被显式写入，就认为调用方要覆盖默认上下文构造逻辑。
	return probe.InvocationContext != nil
}
