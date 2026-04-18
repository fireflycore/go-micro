package invocation

import (
	"context"

	"google.golang.org/grpc"
)

// UnaryInvokeFunc 表示底层 unary 调用函数。
//
// 通过把真实调用抽象成函数，
// 可以让 UnaryInvoker 在测试中替换掉真实的 grpc.ClientConn.Invoke。
type UnaryInvokeFunc func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error

// InvokeOptions 表示一次调用的附加配置。
type InvokeOptions struct {
	// InvocationContext 表示本次调用要附带的统一上下文。
	InvocationContext *InvocationContext
	// CallOptions 表示附加的 gRPC CallOption。
	CallOptions []grpc.CallOption
}

// InvokeOption 表示单个调用选项。
type InvokeOption func(*InvokeOptions)

// WithInvocationContext 设置本次调用使用的 InvocationContext。
func WithInvocationContext(invocation *InvocationContext) InvokeOption {
	return func(options *InvokeOptions) {
		options.InvocationContext = invocation
	}
}

// WithCallOptions 追加底层 gRPC CallOption。
func WithCallOptions(callOptions ...grpc.CallOption) InvokeOption {
	return func(options *InvokeOptions) {
		options.CallOptions = append(options.CallOptions, callOptions...)
	}
}

func buildInvokeOptions(options ...InvokeOption) InvokeOptions {
	// 默认给一份零值调用上下文，避免调用路径上出现 nil 解引用。
	out := InvokeOptions{
		InvocationContext: &InvocationContext{},
	}
	for _, apply := range options {
		if apply == nil {
			continue
		}
		apply(&out)
	}
	return out
}

// UnaryInvoker 是默认的 Invoker 实现。
//
// 它的执行流程非常明确：
// 1. 基于 ServiceDNS 获取连接；
// 2. 基于 InvocationContext 构造统一 metadata；
// 3. 在真正发起 gRPC 调用前执行 Authz 判定；
// 4. 使用 grpc.ClientConn.Invoke 发起 unary 调用。
type UnaryInvoker struct {
	// Dialer 负责连接获取与复用。
	Dialer Dialer
	// Authorizer 是可选依赖；若为空，则跳过 Authz 预检查。
	Authorizer Authorizer
	// InvokeFunc 是可选依赖；若为空，则默认调用 grpc.ClientConn.Invoke。
	InvokeFunc UnaryInvokeFunc
}

// Invoke 执行一次标准 unary 调用。
func (u *UnaryInvoker) Invoke(ctx context.Context, service *ServiceDNS, method string, req any, resp any, options ...InvokeOption) error {
	if u == nil || u.Dialer == nil {
		return ErrInvokerDialerIsNil
	}
	if method == "" {
		return ErrInvokeMethodEmpty
	}

	invokeOptions := buildInvokeOptions(options...)
	// 再兜底一次，确保后续逻辑拿到的调用上下文一定非 nil。
	if invokeOptions.InvocationContext == nil {
		invokeOptions.InvocationContext = &InvocationContext{}
	}

	// 先把本次调用的身份、方法和 metadata 统一成 AuthzContext。
	authzContext := NewAuthzContext(service, method, invokeOptions.InvocationContext)
	if u.Authorizer != nil {
		if err := u.Authorizer.Authorize(ctx, authzContext); err != nil {
			return err
		}
	}

	// 然后按业务服务 DNS 获取可复用连接。
	conn, err := u.Dialer.Dial(ctx, service)
	if err != nil {
		return err
	}

	// 最后把调用上下文转成出站 metadata 上下文。
	outCtx, cancel := invokeOptions.InvocationContext.NewOutgoingContext(ctx)
	defer cancel()

	invokeFunc := u.InvokeFunc
	if invokeFunc == nil {
		invokeFunc = func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			return conn.Invoke(ctx, method, req, resp, options...)
		}
	}

	return invokeFunc(outCtx, conn, method, req, resp, invokeOptions.CallOptions...)
}
