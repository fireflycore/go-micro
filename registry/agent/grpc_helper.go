package agent

import (
	"errors"
	"fmt"
	"strings"

	baseregistry "github.com/fireflycore/go-micro/registry"
	"google.golang.org/grpc"
)

// GRPCDescriptorOptions 描述如何从 gRPC 服务描述直接构造 agent 注册信息。
type GRPCDescriptorOptions struct {
	// AppID 表示应用标识。
	AppID string
	// AppName 表示应用名称；为空时回退到 ServiceName。
	AppName string
	// ServiceName 表示逻辑服务名。
	ServiceName string
	// Namespace 表示命名空间。
	Namespace string
	// DNS 表示统一服务域名。
	DNS string
	// Env 表示运行环境。
	Env string
	// Port 表示业务监听端口。
	Port int
	// Protocol 表示业务协议。
	Protocol string
	// Version 表示业务版本号。
	Version string
	// ServiceConf 表示 go-micro 已有的服务配置。
	ServiceConf *baseregistry.ServiceConf
	// RawServices 表示业务已注册的 gRPC ServiceDesc 集合。
	RawServices []*grpc.ServiceDesc
}

// NewRegistryDescriptorFromGRPC 使用 gRPC ServiceDesc 与 go-micro ServiceConf 构造 registry 描述。
func NewRegistryDescriptorFromGRPC(options GRPCDescriptorOptions) (RegistryDescriptor, error) {
	// 服务名与端口是构造 sidecar-agent register 请求的最小前提。
	if strings.TrimSpace(options.ServiceName) == "" {
		return RegistryDescriptor{}, errors.New("service name is required")
	}
	if options.Port <= 0 {
		return RegistryDescriptor{}, errors.New("port is required")
	}
	// 创建 go-micro 节点描述，并从 ServiceConf 中补齐默认值。
	node := &baseregistry.ServiceNode{
		Methods: make(map[string]bool),
		Meta: &baseregistry.ServiceMeta{
			AppId:   strings.TrimSpace(options.AppID),
			Version: strings.TrimSpace(options.Version),
			Env:     strings.TrimSpace(options.Env),
		},
	}
	if options.ServiceConf != nil {
		// Bootstrap 会补齐 kernel、weight 等默认值。
		options.ServiceConf.Bootstrap()
		node.Weight = options.ServiceConf.Weight
		node.Kernel = options.ServiceConf.Kernel
		node.Meta.InstanceId = strings.TrimSpace(options.ServiceConf.InstanceId)
	}
	// 逐个解析 gRPC service desc，转换为统一 method 路径。
	for _, desc := range options.RawServices {
		if desc == nil {
			continue
		}
		for _, method := range desc.Methods {
			node.Methods[fmt.Sprintf("/%s/%s", desc.ServiceName, method.MethodName)] = true
		}
	}
	// 组装 sidecar-agent registry 描述，供后续运行时直接复用。
	return RegistryDescriptor{
		AppID:       strings.TrimSpace(options.AppID),
		AppName:     strings.TrimSpace(options.AppName),
		ServiceName: strings.TrimSpace(options.ServiceName),
		Namespace:   strings.TrimSpace(options.Namespace),
		DNS:         strings.TrimSpace(options.DNS),
		Env:         strings.TrimSpace(options.Env),
		Port:        options.Port,
		Protocol:    strings.TrimSpace(options.Protocol),
		Version:     strings.TrimSpace(options.Version),
		Node:        node,
	}, nil
}

// NewServiceLifecycleFromGRPC 使用 gRPC ServiceDesc 与 ServiceConf 直接创建业务生命周期桥接对象。
func NewServiceLifecycleFromGRPC(descriptorOptions GRPCDescriptorOptions, runtimeOptions LocalRuntimeOptions, lifecycleOptions LifecycleOptions) (*ServiceLifecycle, error) {
	// 先把 gRPC 描述转换成统一 registry 描述。
	descriptor, err := NewRegistryDescriptorFromGRPC(descriptorOptions)
	if err != nil {
		return nil, err
	}
	// 再复用已有入口组装完整生命周期桥接对象。
	return NewServiceLifecycleFromRegistry(descriptor, runtimeOptions, lifecycleOptions)
}
