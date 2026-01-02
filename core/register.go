package micro

import (
	"fmt"

	"google.golang.org/grpc"
)

// Register 定义服务注册器的最小能力集合。
type Register interface {
	Install(service *ServiceNode) error
	Uninstall()
	SustainLease()
	WithRetryBefore(func())
	WithRetryAfter(func())
	WithLog(func(level LogLevel, message string))
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
