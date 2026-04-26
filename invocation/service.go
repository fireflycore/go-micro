package invocation

import (
	"context"
	"strings"

	svc "github.com/fireflycore/go-micro/service"
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
	services map[string]*svc.DNS
}

// NewRemoteServiceManaged 创建一个轻量的多业务服务装配器。
func NewRemoteServiceManaged(invoker *UnaryInvoker, services ...svc.DNS) *RemoteServiceManaged {
	registered := make(map[string]*svc.DNS, len(services))
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
func (r *RemoteServiceManaged) DNS(serviceName string) (*svc.DNS, error) {
	if r == nil {
		return nil, ErrRemoteServiceNotFound
	}
	dns, ok := r.services[strings.TrimSpace(serviceName)]
	if !ok || dns == nil {
		return nil, ErrRemoteServiceNotFound
	}
	cloned := *dns
	return &cloned, nil
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
	caller, err := r.Caller(serviceName)
	if err != nil {
		return err
	}
	return caller.Invoke(ctx, method, req, resp, callOptions...)
}
