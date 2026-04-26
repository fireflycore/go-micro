package invocation

import (
	"context"
	"strings"
	"time"

	"github.com/fireflycore/go-micro/constant"
	svc "github.com/fireflycore/go-micro/service"
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
	Dial(ctx context.Context, dns *svc.DNS) (*grpc.ClientConn, error)
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
	Invoke(ctx context.Context, dns *svc.DNS, method string, req any, resp any, callOptions ...grpc.CallOption) error
}

// UnaryInvokeFunc 表示底层 unary 调用函数。
//
// 通过把真实调用抽象成函数，
// 可以让 UnaryInvoker 在测试中替换掉真实的 grpc.ClientConn.Invoke。
type UnaryInvokeFunc func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error

// UnaryInvoker 是默认的 Invoker 实现。
//
// 它的执行流程非常明确：
// 1. 基于 service.DNS 获取连接；
// 2. 基于当前链路 metadata 构造统一出站上下文，并注入服务自身身份；
// 3. 使用 grpc.ClientConn.Invoke 发起 unary 调用。
type UnaryInvoker struct {
	// Dialer 负责连接获取与复用。
	Dialer Dialer
	// ServiceAppId 表示当前发起调用的服务应用标识。
	ServiceAppId string
	// ServiceInstanceId 表示当前发起调用的服务实例标识。
	ServiceInstanceId string
	// Timeout 表示统一的远程调用超时时间；未设置时默认 5s。
	Timeout time.Duration
	// InvokeFunc 是可选依赖；若为空，则默认调用 grpc.ClientConn.Invoke。
	InvokeFunc UnaryInvokeFunc
}

// NewUnaryInvoker 创建一个带服务自身身份配置的统一调用器。
func NewUnaryInvoker(dialer Dialer, serviceAppId string, serviceInstanceId string, timeout time.Duration) *UnaryInvoker {
	return &UnaryInvoker{
		Dialer:            dialer,
		ServiceAppId:      serviceAppId,
		ServiceInstanceId: serviceInstanceId,
		Timeout:           normalizeInvokeTimeout(timeout),
	}
}

// Invoke 执行一次标准 unary 调用。
func (u *UnaryInvoker) Invoke(ctx context.Context, dns *svc.DNS, method string, req any, resp any, callOptions ...grpc.CallOption) error {
	if u == nil || u.Dialer == nil {
		return ErrInvokerDialerIsNil
	}
	if method == "" {
		return ErrInvokeMethodEmpty
	}

	// 直接复用当前链路 metadata，并注入当前服务自身身份。
	resolvedMetadata := resolveOutgoingMetadata(ctx, u.ServiceAppId, u.ServiceInstanceId)

	// 然后按业务服务 DNS 获取可复用连接。
	conn, err := u.Dialer.Dial(ctx, dns)
	if err != nil {
		return err
	}

	// 最后把调用上下文转成出站 metadata 上下文。
	outCtx, cancel := NewOutgoingCallContext(ctx, resolvedMetadata, u.Timeout)
	defer cancel()

	invokeFunc := u.InvokeFunc
	if invokeFunc == nil {
		invokeFunc = func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			return conn.Invoke(ctx, method, req, resp, options...)
		}
	}

	return invokeFunc(outCtx, conn, method, req, resp, callOptions...)
}

// resolveOutgoingMetadata 从当前链路解析最终出站 metadata，并注入当前服务身份字段。
//
// 优先读取 incoming metadata，是因为最常见场景是服务端处理请求后继续发起下游调用；
// 若当前已经位于客户端调用链，则退化为复用现有 outgoing metadata。
func resolveOutgoingMetadata(ctx context.Context, serviceAppId string, serviceInstanceId string) metadata.MD {
	var md metadata.MD
	if incoming, ok := metadata.FromIncomingContext(ctx); ok {
		md = incoming.Copy()
	} else if outgoing, ok := metadata.FromOutgoingContext(ctx); ok {
		md = outgoing.Copy()
	} else {
		md = metadata.New(nil)
	}
	if value := strings.TrimSpace(serviceAppId); value != "" {
		md.Set(constant.ServiceAppId, value)
	}
	if value := strings.TrimSpace(serviceInstanceId); value != "" {
		md.Set(constant.ServiceInstanceId, value)
	}
	return md
}

// NewOutgoingCallContext 基于父 context、metadata 和 timeout 构造新的 gRPC 出站 context。
//
// 这里保留父 context 的取消信号与已有 deadline，
// 避免像旧实现那样切断上游取消传播。
func NewOutgoingCallContext(parent context.Context, md metadata.MD, timeout time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		// 调用侧若未提供父 context，则退化为 Background，保证返回值始终可用。
		parent = context.Background()
	}
	if md == nil {
		// gRPC metadata 上下文要求可写的 metadata 容器，这里统一兜底一份空 map。
		md = metadata.New(nil)
	} else {
		// 防御式复制，避免外部继续修改 md 时影响当前调用。
		md = md.Copy()
	}

	ctx := metadata.NewOutgoingContext(parent, md)
	return context.WithTimeout(ctx, normalizeInvokeTimeout(timeout))
}

func normalizeInvokeTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return DefaultInvokeTimeout
	}
	return timeout
}
