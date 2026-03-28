# Invocation

`invocation` 包定义了 Firefly 新一代的服务调用模型。

它不再围绕“节点注册中心”组织能力，而是围绕下面几个问题组织能力：

- 我要调用哪个服务
- 如何把服务身份解析为最终目标
- 如何复用连接
- 如何统一注入 metadata
- 如何在调用前接入 Authz

## 包定位

`invocation` 是新的主路径能力，适合承载：

- `K8s + Istio` 标准实现
- `etcd / consul` 轻量实现
- 统一的 `service -> service` 调用模型

它不再承担旧 `registry` 中的中心语义，例如：

- `ServiceNode`
- `Network`
- `Discovery(method -> nodes)`

## 核心概念

### ServiceRef

`ServiceRef` 用于表达一次调用面向的服务身份。

当前字段包括：

- `Service`
- `Namespace`
- `Env`
- `Port`

其中：

- `Service + Namespace` 主要参与目标地址生成
- `Env` 更偏向环境与策略域语义
- `Port` 作为可选覆盖项存在，默认建议由核心库补齐

### Target

`Target` 表示最终可拨号目标。

默认构造规则是：

```text
<service>.<namespace>.svc.cluster.local:<port>
```

例如：

```text
auth.default.svc.cluster.local:9000
```

### ServiceEndpoint

`ServiceEndpoint` 表示底层实例级端点。

注意：

- 它只服务于基础设施层
- 不作为业务调用侧主模型

### Locator

`Locator` 负责把 `ServiceRef` 解析成 `Target`。

标准实现下，它可以只是一个简单的目标构造器；  
轻量实现下，它也可以在内部维护缓存、watch 和服务地址解析逻辑。

### Dialer

`Dialer` 负责把 `ServiceRef` 转成 `grpc.ClientConn`。

### Invoker

`Invoker` 是统一调用入口，负责把：

- 服务身份
- 调用上下文
- Authz 预检查
- 连接获取
- 实际 gRPC 调用

串起来。

### InvocationContext

`InvocationContext` 表示一次调用附带的统一上下文，例如：

- trace
- metadata
- 调用方身份
- timeout / deadline

### AuthzContext

`AuthzContext` 表示外挂 Authz 所需的标准化输入，例如：

- 调用者是谁
- 调用目标服务是谁
- 调用的完整 method 是什么
- 当前身份元信息是什么

## 当前默认实现

当前版本已经提供：

- `StaticLocator`
- `ConnectionManager`
- `UnaryInvoker`

它们共同构成一个最小可用的调用模型闭环：

1. 通过 `ServiceRef` 标识目标服务
2. 通过 `Locator` 解析目标
3. 通过 `ConnectionManager` 缓存连接
4. 通过 `UnaryInvoker` 执行 Authz + metadata + gRPC 调用

## 示例

```go
package example

import (
	"context"
	"time"

	"github.com/fireflycore/go-micro/invocation"
)

func Example() error {
	locator := invocation.StaticLocator{
		Options: invocation.TargetOptions{
			DefaultPort: 9000,
		},
	}

	manager, err := invocation.NewConnectionManager(invocation.ConnectionManagerOptions{
		Locator: locator,
	})
	if err != nil {
		return err
	}
	defer func() { _ = manager.Close() }()

	invoker := &invocation.UnaryInvoker{
		Dialer: manager,
	}

	ref := invocation.ServiceRef{
		Service:   "auth",
		Namespace: "default",
		Env:       "dev",
	}

	return invoker.Invoke(
		context.Background(),
		ref,
		"/acme.auth.v1.AuthService/Check",
		&struct{}{},
		&struct{}{},
		invocation.WithInvocationContext(invocation.InvocationContext{
			Timeout: 3 * time.Second,
		}),
	)
}
```

## 设计约束

- 业务侧优先面向 `ServiceRef`，而不是节点列表
- `invocation` 不承载后端专属实现细节
- `Authz` 默认作为调用前外挂能力接入
- `K8s + Istio` 是标准路径
- `etcd / consul` 应实现相同的调用语义，而不是暴露另一套模型
