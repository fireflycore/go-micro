package invocation

import (
	"context"
	"strings"

	"google.golang.org/grpc"
)

// RemoteServiceManaged 管理多组远程业务服务 DNS，并复用同一个调用主线。
//
// 这个对象只解决三件事：
// - 统一登记多组远程业务服务；
// - 统一复用同一个 UnaryInvoker；
// - 按业务服务名派生 caller 或直接发起 full method 调用。
type RemoteServiceManaged struct {
	invoker  *UnaryInvoker
	services map[string]*DNS
}

// NewRemoteServiceManaged 创建一个轻量的多业务服务装配器。
func NewRemoteServiceManaged(invoker *UnaryInvoker, services ...DNS) *RemoteServiceManaged {
	registered := make(map[string]*DNS, len(services))
	for _, item := range services {
		name := strings.TrimSpace(item.Service)
		if name == "" {
			continue
		}
		dns := item
		registered[name] = &dns
	}
	return &RemoteServiceManaged{
		invoker:  invoker,
		services: registered,
	}
}

// DNS 返回指定业务服务名对应的 DNS 副本。
func (r *RemoteServiceManaged) DNS(serviceName string) (*DNS, error) {
	// 先定位内部登记的原始 DNS 指针。
	dns, err := r.lookup(serviceName)
	if err != nil {
		return nil, err
	}
	// 对外返回副本，避免调用方修改内部注册表。
	cloned := *dns
	return &cloned, nil
}

// lookup 返回内部注册表中的原始 DNS 指针，仅供内部热路径复用。
func (r *RemoteServiceManaged) lookup(serviceName string) (*DNS, error) {
	if r == nil {
		return nil, ErrRemoteServiceNotFound
	}
	// 统一对服务名做空白裁剪，减少调用方差异带来的查询失败。
	dns, ok := r.services[strings.TrimSpace(serviceName)]
	if !ok || dns == nil {
		return nil, ErrRemoteServiceNotFound
	}
	// 返回内部保存的稳定指针，避免额外复制。
	return dns, nil
}

// Caller 为指定业务服务派生一个复用主调用器的 RemoteServiceCaller。
func (r *RemoteServiceManaged) Caller(serviceName string) (*RemoteServiceCaller, error) {
	dns, err := r.DNS(serviceName)
	if err != nil {
		return nil, err
	}
	return NewRemoteServiceCaller(r.invoker, dns), nil
}

// Invoke 按业务服务名直接发起一次标准 unary 调用。
func (r *RemoteServiceManaged) Invoke(ctx context.Context, serviceName string, method string, req any, resp any, callOptions ...grpc.CallOption) error {
	if r == nil || r.invoker == nil {
		return ErrInvokerDialerIsNil
	}
	// 内部直取已注册的 DNS 指针，避免经 Caller() 再构造一层包装对象。
	dns, err := r.lookup(serviceName)
	if err != nil {
		return err
	}
	// 直接复用同一个 invoker 发起调用。
	return r.invoker.Invoke(ctx, dns, method, req, resp, callOptions...)
}
