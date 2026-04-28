# Logger

`logger` 包提供基于 zap 的日志封装，并通过 `otelzap` 把日志桥接到 OpenTelemetry。

## 功能

- Console：输出到 stdout，面向本地开发和人工排障
- Remote：输出到 OpenTelemetry，面向集中采集、检索与关联分析

## 设计边界

- `logger` 不再依赖业务服务的 `BootstrapConfig` 接口或 `app.Config` 结构。
- `logger` 初始化只接收自己需要的最小输入：
  - `appName string`
  - `logger.Config`
- 业务服务负责在启动层把自己的引导配置映射为 `appName` 和 `logger.Config`，再调用本库。

这意味着：

- 业务侧可以继续保留自己的 `BootstrapConfig`
- `logger` 不需要 import 业务侧启动配置模型
- 配置模型和初始化能力边界更清晰

## 初始化方式

当前入口为：

```go
func NewZapLogger(appName string, config *Config) *zap.Logger
```

其中：

- `appName` 用于标识当前服务，供 `otelzap` bridge 使用
- `config.Console=true` 时启用 console 输出
- `config.Remote=true` 时启用 OpenTelemetry 输出

## 使用示例

```go
package main

import (
	"context"

	"github.com/fireflycore/go-micro/logger"
	"go.uber.org/zap"
)

func main() {
	cfg := &logger.Config{
		Console: true,
		Remote:  true,
	}

	zl := logger.NewZapLogger("order-service", cfg)
	log := logger.NewAccessLogger(zl)

	log.WithContextInfo(context.Background(), "hello", zap.String("k", "v"))
}
```

## 与业务启动配置的关系

推荐在业务服务自己的启动配置中聚合各库配置，例如：

```go
type BootstrapConfig struct {
	App struct {
		Name string `json:"name"`
	} `json:"app"`

	Logger logger.Config `json:"logger"`
}
```

然后在组合根中做一次映射：

```go
zl := logger.NewZapLogger(conf.App.Name, &conf.Logger)
```

这种做法的核心是：

- 业务服务拥有自己的启动配置模型
- 基础库只接收最小输入
- 配置聚合和依赖装配都放在业务服务启动层完成

## Trace 关联

当启用 Remote 输出且服务已初始化 OpenTelemetry Logs Provider 后：

- 如果日志 fields 中包含 `zap.Any("ctx", ctx)`，`otelzap` 会从 `ctx` 中提取 span context，并关联到 OTLP log record
- `AccessLogger` 和 `ServerLogger` 的 `WithContextInfo` / `WithContextWarn` / `WithContextError` 已内置该字段注入
