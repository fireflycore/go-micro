# Logger

`logger` 包提供基于 zap 的日志封装，并通过 `otelzap` 将日志输出到 OpenTelemetry。

## 功能

- Console：输出到 stdout（面向人读）
- Remote：输出到 OpenTelemetry（面向机器采集/检索）

## 使用

`NewZapLogger` 需要一个实现了 `config.BootstrapConfig` 接口的配置对象。

```go
import (
	"context"

	"github.com/fireflycore/go-micro/config"
	"github.com/fireflycore/go-micro/logger"
	"go.uber.org/zap"
)

// MyConf 实现了 config.BootstrapConfig
type MyConf struct {
	AppName string
	Logger  *logger.Conf
}

func (c *MyConf) GetAppName() string { return c.AppName }
func (c *MyConf) GetLoggerConsole() bool { return c.Logger.GetLoggerConsole() }
func (c *MyConf) GetLoggerRemote() bool { return c.Logger.GetLoggerRemote() }
// ... 实现其他接口方法 ...

func main() {
	myConf := &MyConf{
		AppName: "your-service",
		Logger:  &logger.Conf{Console: true, Remote: true},
	}
	
	// 注意：实际项目中还需要实现 config.BootstrapConfig 的其他方法
	zl := logger.NewZapLogger(myConf)
	log := logger.NewAccessLogger(zl)

	log.WithContextInfo(context.Background(), "hello", zap.String("k", "v"))
}
```

### Trace 关联

当启用 Remote 输出（`otelzap.NewCore(...)`）且服务已初始化 OpenTelemetry Logs Provider 后：
- 在日志 fields 中包含 `zap.Any("ctx", ctx)`，`otelzap` 会从 `ctx` 提取 span context 并关联到 OTLP log record。
- 本库的 `WithContextInfo/WithContextWarn/WithContextError` 已内置该字段注入。
