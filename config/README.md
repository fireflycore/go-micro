# Config

`config` 包定义了统一配置能力的核心契约，目标是让业务只依赖 `go-micro/config`，再由具体后端实现包提供存储与监听能力。

> 当前主线口径：统一的是契约、读取语义与控制面边界，不是把多个后端实现同时打进一个运行时产物。当前交付主线中，IDC 使用 `go-consul/config`，`K8s + Istio` 使用 `go-k8s/config`。

## 设计目标

- 统一配置模型，避免不同后端字段语义漂移
- 统一存储与监听接口，降低后端切换成本
- 统一选项与错误语义，减少重复治理逻辑
- 支持运行时按上下文读取（TenantId/AppId/UserId）
- 统一加密语义：一份配置要加密就整份加密，读取时按 `Item.Encrypted` 决定是否先解密再解析

## 目录说明

- `model.go`：配置主键、配置内容、版本元数据、查询上下文、监听事件模型
- `store.go`：统一存储接口
- `watch.go`：统一监听接口
- `option.go`：函数式配置选项与可插拔能力（Codec/Encryptor）
- `loader.go`：统一配置加载入口，负责按 local / remote 方式加载基础配置，并支持从 `Store` 读取配置对象
- `loader_qa.md`：记录 loader 命名与参数设计上的关键取舍，避免后续语义漂移
- `error.go`：统一错误定义

## 命名约定

- `BootstrapConfig` 是业务服务自己的启动配置模型，表示程序启动时只初始化一次的基础引导配置，这类配置通常不参与热更新
- `go-micro/config` 不定义具体业务侧的 `BootstrapConfig` 结构，而是提供通用加载能力，因此这里使用 `loader` 命名
- `LoaderParams` + `LoadConfig` 用于描述“如何从 local / remote 加载一份基础配置”
- `StoreParams` + `LoadStoreConfig` 用于描述“如何从统一配置存储中读取并解析一份配置”
- `Item.Encrypted` 表示“当前整份配置内容是否为密文”，不支持字段级加密
- 这样区分后，业务侧可以继续保留 `BootstrapConfig` 语义，基础库侧则专注于加载过程，避免把“配置模型”和“加载动作”混在一起
- 关于为什么当前不把 `LoadConfig` 的参数进一步收敛成统一接口，可参考 [loader_qa.md](file:///Users/lhdht/product/firefly/go-micro/config/loader_qa.md)

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
- 场景层：按 `TenantId/AppId/UserId` 查询

### 4) 加密读取规则

- `Item.Encrypted=false`：直接解析配置内容
- `Item.Encrypted=true`：先解密整份配置内容，再解析目标结构
- 如果只有部分敏感信息需要保护，应拆成独立配置项，而不是在同一份 JSON 中做字段级加密

## 最小示例

```go
package main

import (
	"context"

	microcfg "github.com/fireflycore/go-micro/config"
)

func load(ctx context.Context, store microcfg.Store) (*microcfg.Item, error) {
	key := microcfg.Key{
		TenantId: "t1",
		Env:      "prod",
		AppId:    "order-service",
		Group:    "db",
		Name:     "primary",
	}
	return store.Get(ctx, key)
}
```
