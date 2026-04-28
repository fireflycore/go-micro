# go-micro `conf` 迁移历史说明

## 文档状态

本文档只保留为历史迁移说明。

旧阶段曾计划把启动配置相关接口从 `github.com/fireflycore/go-micro/conf` 收敛到 `github.com/fireflycore/go-micro/config` 根包；但当前主线已经进一步收敛：

- `go-micro/config` 只保留统一配置契约、loader 语义和错误语义
- 业务服务自己的启动配置模型由业务侧自行定义
- `logger`、`telemetry` 等基础库只接收最小输入，不再通过公共启动配置接口耦合

因此，本文不再作为当前设计主线，只用于帮助识别旧代码中的 `conf` 依赖。

## 当前有效口径

### 1. `go-micro/config` 保留什么

当前 `go-micro/config` 负责：

- `Key`
- `Raw`
- `WatchEvent`
- `Store`
- `Watcher`
- `LoaderParams`
- `StoreParams`
- `LoadConfig(...)`
- `LoadStoreConfig(...)`
- 统一错误语义与解码语义

### 2. 业务服务自己保留什么

业务服务自己定义启动配置聚合模型，例如：

```go
type BootstrapConfig struct {
	App       app.Config       `json:"app"`
	Logger    logger.Config    `json:"logger"`
	Service   service.Config   `json:"service"`
	Telemetry telemetry.Config `json:"telemetry"`
}
```

### 3. 基础库当前推荐接入方式

`logger`：

```go
zl := logger.NewZapLogger(conf.App.Name, &conf.Logger)
```

其中 `conf.Logger` 当前直接使用结构体字段，例如：

```go
type Config struct {
	Console bool `json:"console"`
	Remote  bool `json:"remote"`
}
```

`telemetry`：

```go
providers, err := telemetry.NewProviders(&conf.Telemetry, &telemetry.Resource{
	ServiceId:         conf.App.Id,
	ServiceName:       conf.Service.Service,
	ServiceVersion:    conf.App.Version,
	ServiceNamespace:  conf.Service.Namespace,
	ServiceInstanceId: conf.App.InstanceId,
})
```

## 如果旧项目仍依赖 `go-micro/conf`

建议按下面顺序迁移：

1. 全局检索旧引用：

```text
github.com/fireflycore/go-micro/conf
BootstrapConf
LoggerConf
TelemetryConf
```

2. 删除对 `go-micro/conf` 的直接依赖。
3. 把原先通过接口暴露的启动配置，改为业务服务自己的聚合结构。
4. 在业务服务启动层完成字段映射，再调用 `logger`、`telemetry` 等基础库。
5. 执行一次完整编译或测试验证。

## 迁移判断原则

- 如果一个类型只是“携带配置数据”，优先保留为结构体
- 如果一个抽象是“可替换行为”，再定义接口
- 不要再把 `BootstrapConfig` 这类业务启动模型继续下沉为基础库公共接口

## 当前与历史的区别

历史方案：

- `conf` -> `config`
- 启动配置接口集中到 `go-micro/config`

当前方案：

- `conf` 不再作为当前命名使用
- `go-micro/config` 只做统一配置契约
- 启动配置模型归业务服务所有
- 基础库初始化签名收敛为最小输入模型
