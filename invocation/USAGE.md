# Invocation Usage

本文档描述 `invocation` 的推荐接入方式，以及 repo 层应如何装配远程业务服务调用。

## 推荐接入模式

推荐把调用装配拆成两层：

1. 启动装配层集中创建 `ConnectionManager / UnaryInvoker / RemoteServiceManaged`
2. repo 初始化时按业务服务名绑定 `RemoteServiceCaller`

这样可以把“远程业务服务注册表”和“repo 级调用入口”拆开：

- 启动装配层维护“本服务依赖哪些远程业务服务”
- repo 层维护“当前 repo 绑定哪个远程业务服务 caller”

## 多业务服务场景

当一个服务依赖多个远程业务服务时，推荐优先使用 `RemoteServiceManaged`。

### 启动装配

```go
package example

import (
	"time"

	"github.com/fireflycore/go-micro/invocation"
	"github.com/fireflycore/go-micro/service"
)

func BuildRemoteServices(manager *invocation.ConnectionManager) *invocation.RemoteServiceManaged {
	invoker := invocation.NewUnaryInvoker(manager, "config", "config-1", 3*time.Second)

	return invocation.NewRemoteServiceManaged(
		invoker,
		service.DNS{Service: "auth"},
		service.DNS{Service: "iam"},
	)
}
```

### repo 绑定 caller

```go
package example

import "github.com/fireflycore/go-micro/invocation"

type AuthRepo struct {
	caller *invocation.RemoteServiceCaller
}

func NewAuthRepo(services *invocation.RemoteServiceManaged) (*AuthRepo, error) {
	caller, err := services.Caller("auth")
	if err != nil {
		return nil, err
	}
	return &AuthRepo{caller: caller}, nil
}
```

### repo 发起调用

```go
package example

import "context"

func (r *AuthRepo) GetAppSecret(ctx context.Context, req any, resp any) error {
	return r.caller.Invoke(
		ctx,
		"/acme.auth.app.v1.AuthAppService/GetAppSecret",
		req,
		resp,
	)
}
```

## 单业务服务场景

如果 repo 已经明确只依赖一个远程业务服务，可以直接创建 `RemoteServiceCaller`。

```go
package example

import (
	"context"
	"time"

	"github.com/fireflycore/go-micro/invocation"
	"github.com/fireflycore/go-micro/service"
)

func ExampleSingleCaller(manager *invocation.ConnectionManager) error {
	caller := invocation.NewRemoteServiceCaller(
		invocation.NewUnaryInvoker(manager, "config", "config-1", 3*time.Second),
		&service.DNS{Service: "auth"},
	)

	return caller.Invoke(
		context.Background(),
		"/acme.auth.app.v1.AuthAppService/GetAppSecret",
		&struct{}{},
		&struct{}{},
	)
}
```

## 推荐目录分工

若项目已经有统一 provider、bootstrap 或 `internal/dep` 层，推荐按下面方式分工：

- 装配层：集中创建 `RemoteServiceManaged`
- `internal/data/rs_*.go`：在 `New*Repo(...)` 中绑定 `RemoteServiceCaller`
- repo 方法：只保留 `full method + req + resp`

## 你应该怎么写

- 在服务启动时集中声明多组远程业务服务 `service.DNS`
- 统一创建一份 `ConnectionManager / UnaryInvoker / RemoteServiceManaged`
- 在 `New*Repo(...)` 中按业务服务名获取 caller
- 通过不同 full method 区分具体 proto 子服务

## 你不应该再怎么写

- 按每个 proto 子服务单独维护一份远程地址配置
- 在调用侧做实例发现
- 在调用侧感知 Consul / K8s 细节
- 在 repo 中自己拼装 metadata / timeout
- 把 `UnaryInvoker` 当成 repo 层首选装配接口

## 出站上下文规则

当前调用模型下：

- `UnaryInvoker` 直接复用当前链路 metadata
- `UnaryInvoker` 在出站前注入 `ServiceAppId` / `ServiceInstanceId`
- timeout 在 `NewUnaryInvoker(...)` 初始化时注入
- 不再暴露 metadata / timeout 的单次调用覆盖能力

## 相关文档

- `README.md`
- `ARCHITECTURE.md`
- `CONTEXT-USAGE.md`
- `TESTING.md`
- `PERFORMANCE.md`
- `TEST_REPORT.md`
