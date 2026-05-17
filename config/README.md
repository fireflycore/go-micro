# Config

`config` 包定义了统一配置能力的核心契约，目标是让业务只依赖 `go-micro/config`，再由具体后端实现包提供存储与监听能力。

当前口径补充：

- `go-micro/config` 只负责统一配置读取、监听、解码与错误语义
- 业务服务自己的启动配置模型仍由业务侧定义
- `logger`、`telemetry` 等基础库只接收最小输入，不再通过 `go-micro/config` 根包暴露启动配置接口
- cache / watch / `manage/client` 的后续重构计划见设计库 `design/config/plan/go-micro-config-manage-client-refactor-plan.md`
- 当前阶段已先补齐 `Client` 与 `ClientOptions` 契约，后端实现待适配层在后续版本补齐

> 当前主线口径：统一的是契约、读取语义与控制面边界，不是把多个后端实现同时打进一个运行时产物。当前交付主线中，IDC 使用 `go-consul/config`，`K8s + Istio` 使用 `go-k8s/config`。

## 设计目标

- 统一配置模型，避免不同后端字段语义漂移
- 统一存储与监听接口，降低后端切换成本
- 统一选项与错误语义，减少重复治理逻辑
- 支持运行时按 `Key` 读取配置
- 统一加密语义：一份配置要加密就整份加密，读取时按 `Raw.Encrypted` 决定是否先解密再解析

## 加密处理规则

配置加密遵循以下原则：

1. **加密粒度**：整份配置，不做字段级加密
2. **标识方式**：通过 `Raw.Encrypted` 字段标识
3. **读取规则**：
   - 统一通过 `LoadStoreConfig` / `UnmarshalPayload` 还原配置内容
   - 无论是否加密，处理顺序都固定为：`Base64 解码 -> 按需解密 -> 解压 -> 反序列化`
4. **写入规则**：
   - 统一通过 `MarshalPayload` / `EncodePayload` 编码配置内容
   - 处理顺序固定为：`压缩 -> 按需加密 -> Base64 编码`

**示例**：
```go
// 读取配置后统一走 payload 还原流程
value, err := config.LoadStoreConfig[DatabaseConfig](ctx, store, config.StoreParams{
    Key:        key,
    AppSecret:  appSecret,
    Compressor: compressor,
    Encryptor:  encryptor,
})
if err != nil {
    return err
}
_ = value
```

## 与业务启动配置的关系

推荐由业务服务自行定义引导配置聚合结构，例如 `config/example/bootstrap.go`：

```go
type BootstrapConfig struct {
	App       app.Config       `json:"app"`
	Kernel    kernel.Config    `json:"kernel"`
	Logger    logger.Config    `json:"logger"`
	Service   service.Config   `json:"service"`
	Telemetry telemetry.Config `json:"telemetry"`
}
```

然后由业务服务在启动层完成装配：

- `go-micro/config` 负责从统一数据面存储读取、监听并解析配置
- `logger` 只接收 `appName + logger.Config`
- `telemetry` 只接收 `telemetry.Config + telemetry.Resource`

例如，`logger.Config` 当前字段就是：

```go
type Config struct {
	Console bool `json:"console"`
	Remote  bool `json:"remote"`
}
```

这种方式可以避免基础库之间互相依赖对方的配置模型。

## 目录说明

- `model.go`：配置主键、配置内容、版本元数据、监听事件模型
- `store.go`：统一存储接口
- `watch.go`：统一监听接口
- `client.go`：统一配置客户端读取接口
- `client_option.go`：`Client` 的运行参数与 watch/cache 开关
- `option.go`：函数式配置选项与可插拔能力（Codec/Encryptor/Compressor）
- `loader.go`：从 `Store` 读取并解析配置对象
- `payload.go`：统一 payload 编码与解码流程
- `error.go`：统一错误定义
- `example/bootstrap.go`：业务服务引导配置聚合示例

## 命名约定

- `BootstrapConfig` 是业务服务自己的启动配置模型，表示程序启动时只初始化一次的基础引导配置，这类配置通常不参与热更新
- `go-micro/config` 不定义具体业务侧的 `BootstrapConfig` 结构，只保留运行时配置读取、监听和 payload 编解码能力
- `StoreParams` + `LoadStoreConfig` 用于描述"如何从统一配置存储中读取并解析一份配置"
- `MarshalPayload` / `UnmarshalPayload` 用于描述"如何按统一规则编码和还原配置内容"
- `Raw.Encrypted` 表示"当前整份配置内容是否为密文"，不支持字段级加密
- `logger`、`telemetry` 等库如果需要启动信息，应由业务服务在组合根做字段映射，而不是把业务启动配置接口下沉到 `go-micro/config`

## 使用方式

### 1) 实现 Store + Watcher

由各后端实现包完成。

当前主线交付中：

- IDC：`go-consul/config`
- `K8s + Istio`：`go-k8s/config`

### 2) 业务侧依赖抽象

业务代码依赖 `Store/Watcher` 接口，不直接依赖具体后端实现。

当后续适配层补齐 `Client` 后，业务侧应优先依赖 `Client.Get`，由基础库统一收敛 cache 与 watch 行为。

### 3) 配置分层建议

- 启动层：本地 `BootstrapConfig`
- 动态层：后端 watch 热更新
- 场景层：按 `Key` 查询

这里的“启动层”是业务服务概念，不是 `go-micro/config` 根包里的公共接口集合。

### 4) payload 读取规则

- `LoadStoreConfig` 统一走 payload 管线，不再保留旧的直读 JSON 分支
- `Raw.Encrypted=false`：执行 `Base64 解码 -> 解压 -> 反序列化`
- `Raw.Encrypted=true`：执行 `Base64 解码 -> 解密 -> 解压 -> 反序列化`
- 如果只有部分敏感信息需要保护，应拆成独立配置项，而不是在同一份 JSON 中做字段级加密

## 最小示例

```go
package main

import (
	"context"

	microcfg "github.com/fireflycore/go-micro/config"
)

func load(ctx context.Context, store microcfg.Store) (*microcfg.Raw, error) {
	key := microcfg.Key{
		TenantId: "t1",
		Env:      "prod",
		AppId:    "order-service",
		Group:    "db",
		Name:     "primary",
	}
	return store.Get(ctx, key)
}
