# Constant

`constant` 包定义了微服务通信中通用的 Header Key 与 Metadata Key。

## 常用常量

这些常量主要用于 gRPC Metadata 或 HTTP Header 中传递上下文信息。

### 身份与追踪
- `TraceId`: 全链路追踪 ID (`trace-id`)
- `Authorization`: 认证令牌 (`authorization`)
- `UserId`: 用户 ID (`user-id`)
- `AppId`: 应用 ID (`app-id`)
- `TenantId`: 租户 ID (`tenant-id`)
- `ClientIp`: 客户端 IP (`client-ip`)

### 客户端信息
- `SystemName` / `SystemVersion`: 系统名称与版本
- `ClientName` / `ClientVersion`: 客户端名称与版本
- `DeviceFormFactor`: 设备形态
- `AppVersion`: 应用版本

### 使用示例

```go
import "github.com/fireflycore/go-micro/constant"

// 从 metadata 获取 TraceId
md, _ := metadata.FromIncomingContext(ctx)
traceId := md.Get(constant.TraceId)
```
