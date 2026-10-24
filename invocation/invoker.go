package invocation

import (
	"context"
	"time"

	"github.com/fireflycore/go-micro/authz"
	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	// DefaultInvokeTimeout 是统一的默认远程调用超时时间。
	DefaultInvokeTimeout = 5 * time.Second
)

// Dialer 定义“如何把业务服务 DNS 变成 grpc.ClientConn”。
//
// 在新模型中，Dialer 只关心：
// - 标准 DNS target 组装结果；
// - 连接复用；
// - gRPC 连接创建。
type Dialer interface {
	// Dial 根据业务服务 DNS 返回可复用的 gRPC 连接。
	Dial(ctx context.Context, dns *DNS) (*grpc.ClientConn, error)
	// Close 释放 Dialer 持有的底层资源。
	Close() error
}

// Invoker 定义统一调用入口。
//
// 与业务代码直接操作 grpc.ClientConn 不同，
// Invoker 允许框架在调用前统一完成：
// - 业务服务 DNS 目标解析；
// - metadata 注入；
// - 底层连接复用。
type Invoker interface {
	// Invoke 对指定远程业务服务发起一次标准 unary 调用。
	Invoke(ctx context.Context, dns *DNS, method string, req any, resp any, callOptions ...grpc.CallOption) error
}

// UnaryInvokeFunc 表示底层 unary 调用函数。
//
// 通过把真实调用抽象成函数，
// 可以让 UnaryInvoker 在测试中替换掉真实的 grpc.ClientConn.Invoke。
type UnaryInvokeFunc func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error

// defaultUnaryInvokeFunc 使用 grpc.ClientConn 的标准 Invoke 实现。
func defaultUnaryInvokeFunc(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
	// 直接透传给 gRPC 官方调用入口。
	return conn.Invoke(ctx, method, req, resp, options...)
}

// UnaryInvoker 是默认的 Invoker 实现。
//
// 它的执行流程非常明确：
// 1. 从当前运行时上下文独立构建出站视图并获取当前服务凭证；
// 2. 基于 DNS 获取连接；
// 3. 使用 grpc.ClientConn.Invoke 发起 unary 调用。
type UnaryInvoker struct {
	// Dialer 负责连接获取与复用。
	Dialer Dialer
	// ServiceAuthorityProvider 负责为当前这一跳获取 X-Firefly-Service-Authority。
	ServiceAuthorityProvider authz.ServiceAuthorityProvider
	// Timeout 表示统一的远程调用超时时间；未设置时默认 5s。
	Timeout time.Duration
	// InvokeFunc 是可选依赖；若为空，则默认调用 grpc.ClientConn.Invoke。
	InvokeFunc UnaryInvokeFunc
}

// NewUnaryInvoker 创建统一调用器。
func NewUnaryInvoker(dialer Dialer, timeout time.Duration) *UnaryInvoker {
	return &UnaryInvoker{
		Dialer:  dialer,
		Timeout: normalizeInvokeTimeout(timeout),
	}
}

// WithServiceAuthorityProvider 为 invoker 装配当前服务的 service authority provider。
func (u *UnaryInvoker) WithServiceAuthorityProvider(provider authz.ServiceAuthorityProvider) *UnaryInvoker {
	// nil receiver 保持链式调用安全。
	if u == nil {
		return nil
	}
	// 业务调用必须装配 provider；凭证获取使用独立的精确绑定调用器。
	u.ServiceAuthorityProvider = provider
	// 返回自身，便于启动装配中链式配置。
	return u
}

// Invoke 执行一次标准 unary 调用；ctx 必须由调用方显式提供，不能为 nil。
func (u *UnaryInvoker) Invoke(ctx context.Context, dns *DNS, method string, req any, resp any, callOptions ...grpc.CallOption) error {
	if u == nil || u.Dialer == nil {
		return ErrInvokerDialerIsNil
	}
	if method == "" {
		return ErrInvokeMethodEmpty
	}
	// 凭证准备、连接获取及远程调用共享同一取消与总时限。
	ctx, cancel := context.WithTimeout(ctx, normalizeInvokeTimeout(u.Timeout))
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}

	// 先确定最终使用的调用函数，避免热路径上每次创建匿名闭包。
	invokeFunc := u.InvokeFunc
	if invokeFunc == nil {
		invokeFunc = defaultUnaryInvokeFunc
	}
	// 每次调用构建白名单，不继承原始 metadata 或上一跳签名。
	resolvedMetadata, err := authz.PrepareOutgoingAuthorityMetadata(ctx, service.OutgoingTarget{
		TargetMethod: constant.RequestMethodGrpcString,
		TargetPath:   method,
	}, u.ServiceAuthorityProvider)
	if err != nil {
		return err
	}

	// 然后按业务服务 DNS 获取可复用连接。
	conn, err := u.Dialer.Dial(ctx, dns)
	if err != nil {
		return err
	}

	// 最后把调用上下文转成出站 metadata 上下文。
	outCtx := metadata.NewOutgoingContext(ctx, resolvedMetadata)

	// 使用最终的 invoke 实现发起调用。
	return invokeFunc(outCtx, conn, method, req, resp, callOptions...)
}

// resolveOutgoingMetadata 复用统一白名单构建，不从原始 transport metadata 回退。
func resolveOutgoingMetadata(ctx context.Context, serviceAuthorityProvider authz.ServiceAuthorityProvider) (metadata.MD, error) {
	return authz.PrepareOutgoingAuthorityMetadata(ctx, service.OutgoingTarget{}, serviceAuthorityProvider)
}

// NewOutgoingCallContext 基于父 context、metadata 和 timeout 构造新的 gRPC 出站 context。
//
// 这里保留父 context 的取消信号与已有 deadline，
// 避免像旧实现那样切断上游取消传播。
// parent 必须非 nil；后台调用应显式传入自己的生命周期 context。
func NewOutgoingCallContext(parent context.Context, md metadata.MD, timeout time.Duration) (context.Context, context.CancelFunc) {
	if md == nil {
		// gRPC metadata 上下文要求可写的 metadata 容器，这里统一兜底一份空 map。
		md = metadata.New(nil)
	} else {
		// 防御式复制，避免外部继续修改 md 时影响当前调用。
		md = md.Copy()
	}

	return newOutgoingCallContextWithOwnedMetadata(parent, md, timeout)
}

func normalizeInvokeTimeout(timeout time.Duration) time.Duration {
	// 非正数统一回落到框架默认超时。
	if timeout <= 0 {
		return DefaultInvokeTimeout
	}
	// 已配置超时时直接返回。
	return timeout
}

// newOutgoingCallContextWithOwnedMetadata 假定 md 已经归当前调用独占持有，不再重复复制。
func newOutgoingCallContextWithOwnedMetadata(parent context.Context, md metadata.MD, timeout time.Duration) (context.Context, context.CancelFunc) {
	// 先把 metadata 挂载到出站上下文。
	ctx := metadata.NewOutgoingContext(parent, md)
	// 再统一施加超时控制。
	return context.WithTimeout(ctx, normalizeInvokeTimeout(timeout))
}
