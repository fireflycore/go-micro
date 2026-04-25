package invocation

import (
	"context"
	"time"

	"github.com/fireflycore/go-micro/constant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Dialer 定义“如何把业务服务 DNS 变成 grpc.ClientConn”。
//
// 在新模型中，Dialer 只关心：
// - 标准 DNS target 组装结果；
// - 连接复用；
// - gRPC 连接创建。
type Dialer interface {
	// Dial 根据业务服务 DNS 返回可复用的 gRPC 连接。
	Dial(ctx context.Context, service *ServiceDNS) (*grpc.ClientConn, error)
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
	Invoke(ctx context.Context, service *ServiceDNS, method string, req any, resp any, options ...InvokeOption) error
}

// UnaryInvokeFunc 表示底层 unary 调用函数。
//
// 通过把真实调用抽象成函数，
// 可以让 UnaryInvoker 在测试中替换掉真实的 grpc.ClientConn.Invoke。
type UnaryInvokeFunc func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error

// InvokeOptions 表示一次调用的附加配置。
type InvokeOptions struct {
	// Metadata 表示本次调用显式补充的出站 metadata。
	//
	// 用户身份等受保护字段不会被业务侧显式值覆写，
	// 最终仍以当前链路已存在的 metadata 为准。
	Metadata metadata.MD
	// Timeout 表示本次调用的显式超时时间。
	Timeout time.Duration
	// CallOptions 表示附加的 gRPC CallOption。
	CallOptions []grpc.CallOption
}

// InvokeOption 表示单个调用选项。
type InvokeOption func(*InvokeOptions)

// WithMetadata 追加本次调用显式补充的出站 metadata。
func WithMetadata(md metadata.MD) InvokeOption {
	return func(options *InvokeOptions) {
		// 多次调用 WithMetadata 时，后面的值覆盖前面的同名字段，
		// 这样调用侧可以按 option 顺序逐步修正显式参数。
		options.Metadata = mergeOptionMetadata(options.Metadata, md)
	}
}

// WithTimeout 设置本次调用的显式超时时间。
func WithTimeout(timeout time.Duration) InvokeOption {
	return func(options *InvokeOptions) {
		options.Timeout = timeout
	}
}

// WithCallOptions 追加底层 gRPC CallOption。
func WithCallOptions(callOptions ...grpc.CallOption) InvokeOption {
	return func(options *InvokeOptions) {
		// CallOption 允许多次叠加，因此这里直接追加。
		options.CallOptions = append(options.CallOptions, callOptions...)
	}
}

// buildInvokeOptions 把可变参数形式的 InvokeOption 收敛成一份最终配置。
func buildInvokeOptions(options ...InvokeOption) InvokeOptions {
	out := InvokeOptions{}
	for _, apply := range options {
		if apply == nil {
			// 允许上游传入 nil，避免调用侧在条件拼装 option 时额外做判空。
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
// 2. 基于继承 metadata 与显式调用选项构造统一出站上下文；
// 3. 使用 grpc.ClientConn.Invoke 发起 unary 调用。
type UnaryInvoker struct {
	// Dialer 负责连接获取与复用。
	Dialer Dialer
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
	// 先基于父 context 里的 metadata 构造一份可安全复用的出站 metadata。
	resolvedMetadata := prepareOutgoingMetadata(ctx, invokeOptions.Metadata)

	// 然后按业务服务 DNS 获取可复用连接。
	conn, err := u.Dialer.Dial(ctx, service)
	if err != nil {
		return err
	}

	// 最后把调用上下文转成出站 metadata 上下文。
	outCtx, cancel := NewOutgoingCallContext(ctx, resolvedMetadata, invokeOptions.Timeout)
	defer cancel()

	invokeFunc := u.InvokeFunc
	if invokeFunc == nil {
		invokeFunc = func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			return conn.Invoke(ctx, method, req, resp, options...)
		}
	}

	return invokeFunc(outCtx, conn, method, req, resp, invokeOptions.CallOptions...)
}

// prepareOutgoingMetadata 组合父 context 中可继承的 metadata 与本次显式 metadata。
func prepareOutgoingMetadata(ctx context.Context, explicit metadata.MD) metadata.MD {
	if explicit != nil {
		// 显式 metadata 必须先复制，避免调用后续修改入参影响本次调用内容。
		explicit = explicit.Copy()
	}
	inherited := inheritedMetadataFromContext(ctx)
	return mergeOutgoingMetadata(inherited, explicit)
}

// inheritedMetadataFromContext 从父 context 提取可沿链路继续透传的 metadata。
//
// 优先读取 incoming metadata，是因为最常见场景是服务端处理请求后继续发起下游调用；
// 若当前已经位于客户端调用链，则退化为复用现有 outgoing metadata。
func inheritedMetadataFromContext(ctx context.Context) metadata.MD {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		return md.Copy()
	}
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		return md.Copy()
	}
	return metadata.New(nil)
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
	if timeout > 0 {
		// 显式 timeout 只在调用侧明确要求时才附加。
		return context.WithTimeout(ctx, timeout)
	}

	// 与 context.WithCancel 的返回值保持一致，统一返回一个可安全调用的 cancel。
	return ctx, func() {}
}

// mergeOutgoingMetadata 把显式 metadata 合并到继承 metadata 上。
//
// 合并规则：
// - 默认先继承链路已有 metadata；
// - 显式 metadata 可以覆盖普通字段；
// - 受保护身份字段始终沿用链路已有值。
func mergeOutgoingMetadata(inherited metadata.MD, explicit metadata.MD) metadata.MD {
	if inherited == nil {
		inherited = metadata.New(nil)
	} else {
		inherited = inherited.Copy()
	}
	if explicit == nil {
		return inherited
	}

	for key, values := range explicit {
		if isProtectedCallerMetadataKey(key) {
			// 用户身份等字段只能透传，不能由业务侧显式覆写。
			continue
		}
		// 使用新切片复制值，避免共享底层数组。
		inherited[key] = append([]string(nil), values...)
	}
	return inherited
}

// mergeOptionMetadata 在多个 WithMetadata 之间合并显式 metadata。
//
// 与最终出站合并不同，这里不处理保护字段，只负责保留调用侧 option 的组合语义。
func mergeOptionMetadata(base metadata.MD, extra metadata.MD) metadata.MD {
	if extra == nil {
		if base == nil {
			return nil
		}
		return base.Copy()
	}

	merged := metadata.New(nil)
	for key, values := range base {
		// 先写入已有 option 的结果。
		merged[key] = append([]string(nil), values...)
	}
	for key, values := range extra {
		// 后续 option 同名字段覆盖前面的值，保持 option 顺序语义。
		merged[key] = append([]string(nil), values...)
	}
	return merged
}

// isProtectedCallerMetadataKey 判断某个 metadata key 是否属于受保护身份字段。
func isProtectedCallerMetadataKey(key string) bool {
	switch key {
	case constant.UserId, constant.AppId, constant.TenantId, constant.OrgIds, constant.RoleIds:
		return true
	default:
		return false
	}
}
