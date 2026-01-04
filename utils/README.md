# Utils

`utils` 包提供通用的工具函数。

## 功能列表

### Network

- `GetInternalNetworkIp()`: 获取本机在局域网中的 IP 地址。
  - 通过建立 UDP 伪连接（不发送数据）来自动选择合适的路由接口 IP。

## 使用示例

```go
import "github.com/fireflycore/go-micro/utils"

ip := utils.GetInternalNetworkIp()
fmt.Println("Local IP:", ip)
```
