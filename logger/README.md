# Logger

`logger` 包定义了基础的日志级别类型。

## 内容

### LogLevel

定义了日志级别字符串类型：

- `Info`: 信息 (`info`)
- `Error`: 错误 (`error`)
- `Success`: 成功 (`success`)

### 使用示例

```go
import "github.com/fireflycore/go-micro/logger"

func Log(level logger.LogLevel, msg string) {
    // ...
}
```
