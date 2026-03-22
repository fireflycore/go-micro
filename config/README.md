# Config

`config` 包定义了统一配置能力的核心契约，目标是让业务只依赖 `go-micro/config`，由 `go-etcd`、`go-consul`、`go-k8s` 提供具体实现。

## 设计目标

- 统一配置模型，避免不同后端字段语义漂移
- 统一存储与监听接口，降低后端切换成本
- 统一选项与错误语义，减少重复治理逻辑
- 支持运行时按上下文读取（Tenant/App/User）

## 目录说明

- `model.go`：配置主键、配置内容、版本元数据、查询上下文、监听事件模型
- `store.go`：统一存储接口
- `watch.go`：统一监听接口
- `option.go`：函数式配置选项与可插拔能力（Codec/Encryptor）
- `error.go`：统一错误定义

## 使用方式

### 1) 实现 Store + Watcher

由各后端实现包完成：

- `go-etcd/config`
- `go-consul/config`
- `go-k8s/config`

### 2) 业务侧依赖抽象

业务代码依赖 `Store/Watcher` 接口，不直接依赖具体后端实现。

### 3) 配置分层建议

- 启动层：本地 `BootstrapConf`
- 动态层：后端 watch 热更新
- 场景层：按 `TenantID/AppID/UserID` 查询

## 最小示例

```go
package main

import (
	"context"

	microcfg "github.com/fireflycore/go-micro/config"
)

func load(ctx context.Context, store microcfg.Store) (*microcfg.Item, error) {
	key := microcfg.Key{
		Tenant: "t1",
		Env:    "prod",
		AppID:  "order-service",
		Group:  "db",
		Name:   "primary",
	}
	return store.Get(ctx, key)
}
```
