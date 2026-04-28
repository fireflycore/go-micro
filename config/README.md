# Config

`config` 包定义了统一配置能力的核心契约，目标是让业务只依赖 `go-micro/config`，再由具体后端实现包提供存储与监听能力。

当前口径补充：

- `go-micro/config` 只负责统一配置读取、监听、解码与错误语义
- 业务服务自己的启动配置模型仍由业务侧定义
- `logger`、`telemetry` 等基础库只接收最小输入，不再通过 `go-micro/config` 根包暴露启动配置接口

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
   - `Raw.Encrypted=false`：直接 JSON 解析 `Content`
   - `Raw.Encrypted=true`：必须先通过 `PayloadDecodeFunc` 解密整份 `Content`，再解析为目标结构
4. **写入规则**：
   - 普通配置：直接写入明文内容，设置 `Encrypted=false`
   - 敏感配置：先加密整份内容，再写入，设置 `Encrypted=true`

**示例**：
```go
// 读取加密配置
raw, err := store.Get(ctx, key)
if err != nil {
    return err
}

if raw.Encrypted {
    // 必须先解密整份内容
    decrypted, err := decrypt(raw.Content, appSecret)
    if err != nil {
        return err
    }
    // 再解析为目标结构
    err = json.Unmarshal(decrypted, &config)
} else {
    // 明文配置直接解析
    err = json.Unmarshal(raw.Content, &config)
}
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

- `go-micro/config` 负责加载本地或远程配置
- `logger` 只接收 `appName + logger.Config`
- `telemetry` 只接收 `telemetry.Config + telemetry.Resource`

这种方式可以避免基础库之间互相依赖对方的配置模型。

## 目录说明

- `model.go`：配置主键、配置内容、版本元数据、监听事件模型
- `store.go`：统一存储接口
- `watch.go`：统一监听接口
- `option.go`：函数式配置选项与可插拔能力（Codec/Encryptor）
- `loader.go`：统一配置加载入口，负责按 local / remote 方式加载基础配置，并支持从 `Store` 读取配置对象
- `loader_qa.md`：记录 loader 命名与参数设计上的关键取舍，避免后续语义漂移
- `error.go`：统一错误定义
- `example/bootstrap.go`：业务服务引导配置聚合示例

## 命名约定

- `BootstrapConfig` 是业务服务自己的启动配置模型，表示程序启动时只初始化一次的基础引导配置，这类配置通常不参与热更新
- `go-micro/config` 不定义具体业务侧的 `BootstrapConfig` 结构，而是提供通用加载能力，因此这里使用 `loader` 命名
- `LoaderParams` + `LoadConfig` 用于描述"如何从 local / remote 加载一份基础配置"
- `StoreParams` + `LoadStoreConfig` 用于描述"如何从统一配置存储中读取并解析一份配置"
- `Raw.Encrypted` 表示"当前整份配置内容是否为密文"，不支持字段级加密
- 这样区分后，业务侧可以继续保留 `BootstrapConfig` 语义，基础库侧则专注于加载过程，避免把"配置模型"和"加载动作"混在一起
- `logger`、`telemetry` 等库如果需要启动信息，应由业务服务在组合根做字段映射，而不是把业务启动配置接口下沉到 `go-micro/config`
- 关于为什么当前不把 `LoadConfig` 的参数进一步收敛成统一接口，可参考 `loader_qa.md`

## 使用方式

### 1) 实现 Store + Watcher

由各后端实现包完成。

当前主线交付中：

- IDC：`go-consul/config`
- `K8s + Istio`：`go-k8s/config`

### 2) 业务侧依赖抽象

业务代码依赖 `Store/Watcher` 接口，不直接依赖具体后端实现。

### 3) 配置分层建议

- 启动层：本地 `BootstrapConfig`
- 动态层：后端 watch 热更新
- 场景层：按 `Key` 查询

这里的“启动层”是业务服务概念，不是 `go-micro/config` 根包里的公共接口集合。

### 4) 加密读取规则

- `Raw.Encrypted=false`：直接解析配置内容
- `Raw.Encrypted=true`：先解密整份配置内容，再解析目标结构
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
