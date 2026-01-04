# Registry

`registry` 包定义了服务注册与发现的核心接口，并提供了多种后端实现。

## 核心接口

- **Register**: 定义服务注册行为（Install, Uninstall）。
- **Discovery**: 定义服务发现行为（GetService, Watcher）。

## 支持的后端

本项目支持以下注册中心实现（位于子目录中）：
- **Consul**: 基于 HashiCorp Consul
- **Etcd**: 基于 Etcd v3
- **Kubernetes**: 基于 K8s API

## 使用示例

### 服务注册

```go
import (
    "github.com/fireflycore/go-micro/registry"
    "github.com/fireflycore/go-micro/registry/consul" // 或其他实现
)

// 1. 初始化具体实现
reg := consul.New(consul.Config{Address: "127.0.0.1:8500"})

// 2. 执行注册
// 自动解析 gRPC ServiceDesc 并注册所有方法路径
errs := registry.NewRegisterService(
    []*grpc.ServiceDesc{&pb.MyService_ServiceDesc},
    reg,
)
```

### 服务发现

```go
// 1. 获取 Discovery 实例
disc := consul.NewDiscovery(consul.Config{...})

// 2. 监听变更（通常在后台运行）
go disc.Watcher()

// 3. 获取服务节点
nodes, err := disc.GetService("MyService")
```
