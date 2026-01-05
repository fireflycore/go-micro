# Constant

`constant` 包定义了微服务通信中通用的 HTTP Header / gRPC Metadata Key。

## 常用常量

这些常量主要用于 gRPC Metadata 或 HTTP Header 中传递上下文信息。

### 身份与追踪
- `TraceId`：全链路追踪 ID（`ff-trace-id`）
- `Session`：会话标识（`ff-session`）
- `Authorization`：认证令牌（`authorization`）
- `UserId`：用户 ID（`ff-user-id`）
- `AppId`：应用 ID（`ff-app-id`）
- `TenantId`：租户 ID（`ff-tenant-id`）
- `ClientIp`：客户端 IP（`ff-client-ip`）
- `XRealIp`：反向代理透传真实 IP（`x-real-ip`）

### 客户端信息
- `SystemName` / `SystemVersion`：系统名称与版本
- `ClientName` / `ClientVersion`：客户端名称与版本
- `SystemType` / `ClientType`：系统/客户端类型
- `DeviceFormFactor`：设备形态
- `AppVersion` / `AppLanguage`：应用版本 / 语言

### 使用示例

```go
import "github.com/fireflycore/go-micro/constant"

// 从 gRPC metadata 获取 TraceId
md, _ := metadata.FromIncomingContext(ctx)
traceId := md.Get(constant.TraceId)
```
