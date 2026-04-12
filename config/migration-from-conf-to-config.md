# go-micro 配置命名迁移说明

## 背景

从下一次 `go-micro` 新版本开始，启动配置相关接口不再单独放在 `github.com/fireflycore/go-micro/conf` 中，而是统一收敛到 `github.com/fireflycore/go-micro/config` 根包。

这次调整的目标只有一个：统一命名。

- 包名统一为 `config`
- 启动配置接口也统一放在 `config` 中
- 不再保留 `conf` 包

## 变更范围

本次变更只涉及“启动配置接口”的包路径与类型命名，不影响 `Store`、`Watcher`、`LoaderParams`、`LoadConfig`、`LoadStoreConfig` 等已有配置契约。

## 迁移摘要

### 包路径变化

| 旧路径 | 新路径 |
| --- | --- |
| `github.com/fireflycore/go-micro/conf` | `github.com/fireflycore/go-micro/config` |

### 类型命名变化

| 旧类型 | 新类型 |
| --- | --- |
| `conf.BootstrapConf` | `config.BootstrapConfig` |
| `conf.LoggerConf` | `config.LoggerConfig` |
| `conf.TelemetryConf` | `config.TelemetryConfig` |

## 不变部分

以下内容保持不变：

- `github.com/fireflycore/go-micro/config` 仍然是统一配置契约主包
- `Key` / `Item` / `Meta` / `Query` / `WatchEvent`
- `Store` / `Watcher`
- `Options` / `Codec` / `Encryptor`
- `LoaderParams` / `StoreParams`
- `LoadConfig(...)` / `LoadStoreConfig(...)`
- `go-consul/config`、`go-k8s/config` 对 `go-micro/config` 的依赖方式

也就是说，这次迁移主要是把原来 `conf` 中的接口并入了 `config` 根包，不是重做整套配置契约。

## 代码改造对照

### 1. import 路径替换

迁移前：

```go
import "github.com/fireflycore/go-micro/conf"
```

迁移后：

```go
import "github.com/fireflycore/go-micro/config"
```

### 2. 接口类型替换

迁移前：

```go
func NewZapLogger(bootstrapConf conf.BootstrapConf) *zap.Logger
```

迁移后：

```go
func NewZapLogger(bootstrapConf config.BootstrapConfig) *zap.Logger
```

### 3. 结构体实现接口时的变化

如果你的结构体只是“实现这些方法”，而没有显式声明 `conf.BootstrapConf` 字段或返回值类型，那么大部分方法实现本身不需要改，只需要：

- 改 import
- 改接口引用类型名

例如：

迁移前：

```go
func NewBootstrapConfImpl(bootstrapConf *BootstrapConf) conf.BootstrapConf {
	return bootstrapConf
}
```

迁移后：

```go
func NewBootstrapConfImpl(bootstrapConf *BootstrapConf) config.BootstrapConfig {
	return bootstrapConf
}
```

## 推荐迁移步骤

建议所有依赖仓库在 `go-micro` 发布新版本后，按下面顺序迁移。

### 第一步：升级 go-micro 版本

先把依赖仓库中的 `github.com/fireflycore/go-micro` 升级到包含本次调整的新版本。

### 第二步：全局替换 import

优先替换旧包路径：

- `github.com/fireflycore/go-micro/conf`
- 替换为 `github.com/fireflycore/go-micro/config`

### 第三步：全局替换类型名

继续替换旧接口名：

- `BootstrapConf` -> `BootstrapConfig`
- `LoggerConf` -> `LoggerConfig`
- `TelemetryConf` -> `TelemetryConfig`

### 第四步：编译与测试

迁移后至少执行：

```bash
go test ./...
```

如果仓库没有完整测试，也至少执行一次：

```bash
go build ./...
```

## 建议的检索关键字

升级下游仓库时，可优先搜索以下关键字：

```text
github.com/fireflycore/go-micro/conf
BootstrapConf
LoggerConf
TelemetryConf
```

## 当前仓库内已知受影响位置

按当前工作区检索，`firefly` 下已知还需要在发布后同步迁移的旧引用包括：

- `go-layout/internal/conf/bootstrap.go`

如果还有其他业务仓库依赖了 `go-micro/conf`，也建议用上面的关键字再做一次全仓检索。

## 兼容性说明

本次调整属于命名与包路径收敛，不是接口方法语义变更。

也就是说：

- 如果你的业务类型已经实现了原先 `BootstrapConf` 所需的方法集合
- 那么迁移后通常不需要重写方法逻辑
- 大多数情况下只需要改 import 和类型名

## 最终口径

后续统一使用以下命名：

- 包：`config`
- 启动配置接口：`BootstrapConfig`
- 日志配置接口：`LoggerConfig`
- 遥测配置接口：`TelemetryConfig`

不再继续使用：

- `conf`
- `BootstrapConf`
- `LoggerConf`
- `TelemetryConf`
