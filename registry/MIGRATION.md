# Registry To Agent Migration

本文档用于后续 AI 或人工批量迁移仓库时参考。

## 当前结论

`go-micro/registry` 根目录下的旧 Go 包已经移除。  
裸机场景统一迁移到：

- `github.com/fireflycore/go-micro/registry/agent`

K8s 场景不应迁移到这套裸机语义。

## 迁移目标

把旧代码中的：

- 通用 `registry.Register`
- 通用 `registry.Discovery`
- `registry.ServiceNode`
- `registry.ServiceConf`
- `registry.NewRegisterService(...)`

迁移为面向 `sidecar-agent` 的裸机接入模型。

## 新主路径

裸机服务统一改用：

- `agent.ServiceRegistration`
- `agent.ServiceRegistrationProvider`
- `agent.ServiceOptions`
- `agent.ServiceNode`
- `agent.ServiceMeta`
- `agent.ServiceKernel`
- `agent.NewServiceRegistrationFromGRPC(...)`
- `agent.NewLocalRuntimeFromServiceRegistration(...)`
- `agent.NewServiceLifecycleFromServiceRegistration(...)`
- `agent.NewServiceLifecycleFromGRPC(...)`
- `agent.ManagedServer`

## 已删除的旧根包文件

- `registry/conf.go`
- `registry/discovery.go`
- `registry/error.go`
- `registry/model.go`
- `registry/register.go`
- `registry/service_method.go`
- `registry/service_node.go`

## 名称映射

| 旧名 | 新名 |
|---|---|
| `registry.ServiceConf` | `agent.ServiceOptions` |
| `registry.ServiceNode` | `agent.ServiceNode` |
| `registry.ServiceMeta` | `agent.ServiceMeta` |
| `registry.ServiceKernel` | `agent.ServiceKernel` |
| `agent.RegistryDescriptor` | `agent.ServiceRegistration` |
| `agent.RegistryProvider` | `agent.ServiceRegistrationProvider` |
| `agent.NewRegistryDescriptorFromGRPC(...)` | `agent.NewServiceRegistrationFromGRPC(...)` |
| `agent.NewLocalRuntimeFromRegistry(...)` | `agent.NewLocalRuntimeFromServiceRegistration(...)` |
| `agent.NewServiceLifecycleFromRegistry(...)` | `agent.NewServiceLifecycleFromServiceRegistration(...)` |

## 代码迁移步骤

### 1. 替换 import

把：

```go
import "github.com/fireflycore/go-micro/registry"
```

改成：

```go
import agent "github.com/fireflycore/go-micro/registry/agent"
```

### 2. 替换模型

把：

```go
&registry.ServiceNode{}
&registry.ServiceMeta{}
&registry.ServiceKernel{}
&registry.ServiceConf{}
```

改成：

```go
&agent.ServiceNode{}
&agent.ServiceMeta{}
&agent.ServiceKernel{}
&agent.ServiceOptions{}
```

### 3. 字段改名

把：

```go
meta.AppId
meta.InstanceId
```

改成：

```go
meta.AppId
meta.InstanceId
```

### 4. 用 sidecar-agent 生命周期替代旧 Register

旧代码如果是：

```go
registry.NewRegisterService(raw, register)
```

不再保留原路径。  
应改成基于：

- `agent.NewServiceRegistrationFromGRPC(...)`
- `agent.NewServiceLifecycleFromGRPC(...)`
- `agent.ManagedServer`

来完成 register / replay / drain / deregister。

## 迁移判断

### 裸机服务

直接迁移到 `registry/agent`。

### K8s 服务

不要迁移到 `registry/agent`，而应改走：

- `invocation`
- `mesh`
- `k8s service discovery`

## 推荐迁移顺序

1. 先替换 import
2. 再替换模型类型与字段名
3. 再替换生命周期入口
4. 最后删除旧注册中心初始化逻辑
