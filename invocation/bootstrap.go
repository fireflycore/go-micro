package invocation

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// BootstrapUnaryInvoker 仅用于精确配置的凭证获取入口，不继承业务身份或任意 metadata。
// 它不使用服务 provider，避免获取服务凭证时递归依赖同一个 provider。
type BootstrapUnaryInvoker struct {
	// dialer 只负责连接获取和复用，不负责业务身份传播。
	dialer Dialer
	// timeout 控制凭证获取总时限，仍保留父级取消与更早期限。
	timeout time.Duration
	// method 是唯一允许调用的完整方法，不能使用前缀匹配。
	method string
	// InvokeFunc 可注入底层调用实现，未指定时使用标准 unary Invoke。
	InvokeFunc UnaryInvokeFunc
}

// NewBootstrapUnaryInvoker 创建独立的凭证获取调用器，不作为普通业务调用回退。
func NewBootstrapUnaryInvoker(dialer Dialer, timeout time.Duration, method string) *BootstrapUnaryInvoker {
	return &BootstrapUnaryInvoker{dialer: dialer, timeout: timeout, method: method}
}

// Invoke 只放行绑定方法，并显式以空 metadata 发起调用。
func (b *BootstrapUnaryInvoker) Invoke(ctx context.Context, dns *DNS, method string, req, resp any, opts ...grpc.CallOption) error {
	if b == nil || b.dialer == nil {
		return ErrInvokerDialerIsNil
	}
	if b.method == "" || b.method != method {
		return errors.New("bootstrap method is not allowed")
	}
	out, cancel := NewOutgoingCallContext(ctx, metadata.MD{}, b.timeout)
	// 关闭当前派生期限，但不取消调用方持有的父 context。
	defer cancel()
	conn, err := b.dialer.Dial(out, dns)
	if err != nil {
		return err
	}
	invoke := b.InvokeFunc
	if invoke == nil {
		invoke = defaultUnaryInvokeFunc
	}
	return invoke(out, conn, method, req, resp, opts...)
}
