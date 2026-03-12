# Logger

`logger` 包提供基于 zap 的日志封装，并通过 `otelzap` 将日志输出到 OpenTelemetry。

## 功能

- Console：输出到 stdout（面向人读）
- Remote：输出到 OpenTelemetry（面向机器采集/检索）

## 使用

```go
import (
	"context"

	"github.com/fireflycore/go-micro/logger"
)

func main() {
	conf := &logger.Conf{Console: true, Remote: true}
	zl := logger.NewZapLogger("your-service", conf)
	log := logger.NewLogger(zl)

	log.Info(context.Background(), "hello")
}
```
