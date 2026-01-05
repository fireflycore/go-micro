package registry

import (
	"fmt"

	"github.com/fireflycore/go-micro/logger"
	"google.golang.org/grpc"
)

// Register 定义服务注册器的最小能力集合。
type Register interface {
	// Install 安装并注册一个服务节点，完成必要的元信息填充与持久化。
	Install(service *ServiceNode) error
	// Uninstall 注销当前注册的服务节点并释放相关资源。
	Uninstall()
	// SustainLease 维持租约/心跳，发生异常时可结合重试策略恢复。
	SustainLease()
	// WithRetryBefore 设置重试前回调，用于指标/告警等场景。
	WithRetryBefore(func())
	// WithRetryAfter 设置重试成功后回调，用于恢复通知等场景。
	WithRetryAfter(func())
	// WithLog 设置内部日志回调，统一输出实现内部状态。
	WithLog(func(level logger.LogLevel, message string))
}

// NewRegisterService 将 gRPC ServiceDesc 解析为节点方法集合并执行注册。
func NewRegisterService(raw []*grpc.ServiceDesc, reg Register) []error {
	if reg == nil {
		return []error{ErrRegisterIsNil}
	}

	node := new(ServiceNode)
	node.ProtoCount = len(raw)
	node.Methods = make(map[string]bool)

	for _, desc := range raw {
		for _, item := range desc.Methods {
			// 方便网关层面快速验证该节点是否有此接口/方法
			node.Methods[fmt.Sprintf("/%s/%s", desc.ServiceName, item.MethodName)] = true
		}
	}

	var errs []error
	if err := reg.Install(node); err != nil {
		errs = append(errs, err)
	}

	return errs
}
