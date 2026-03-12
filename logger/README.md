# Logger

`logger` 包提供基于 zap 的日志封装，并通过 `otelzap` 将日志输出到 OpenTelemetry。

## 功能

- Console：输出到 stdout（面向人读）
- Remote：输出到 OpenTelemetry（面向机器采集/检索）

## 使用

`NewZapLogger` 需要一个实现了 `conf.BootstrapConf` 接口的配置对象。

```go
import (
	"context"

	"github.com/fireflycore/go-micro/logger"
)

// MyConf 实现了 conf.BootstrapConf
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
	
	// 注意：实际项目中还需要实现 conf.BootstrapConf 的其他方法
	zl := logger.NewZapLogger(myConf)
	log := logger.NewLogger(zl)

	log.Info(context.Background(), "hello")
}
```
