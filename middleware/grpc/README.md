# gRPC Middleware (gm)

`gm` 包提供了构建 gRPC 微服务所需的通用中间件（Interceptor）。

## 功能列表

### 1. Access Logger (`NewServiceAccessLogger`)

提供详细的 gRPC 访问日志记录功能，支持结构化日志和人类可读日志的输出。

**特性**：
- **全链路追踪**：自动关联 `TraceId`（若缺失则自动生成）。
- **身份识别**：
  - **Trace Identity**: `SourceIp`, `ClientIp`
  - **Invoke Identity**: `InvokeServiceAppId`, `InvokeServiceEndpoint` (调用方信息)
  - **Target Identity**: `TargetServiceAppId`, `TargetServiceEndpoint` (目标服务信息)
  - **User/App Identity**: `UserId`, `AppId`, `TenantId`
- **客户端信息**：自动提取 `System` (Name, Ver, Type), `Client` (Name, Ver, Type), `DeviceFormFactor` 等元数据。
- **性能监控**：记录请求/响应耗时 (`Duration`) 和状态 (`Status`)。
- **载荷记录**：记录 Request/Response JSON 载荷（用于调试）。

**用法**：

```go
import (
	"fmt"
	
	"github.com/fireflycore/go-micro/middleware/grpc" // 别名通常建议为 gm
	"google.golang.org/grpc"
)

// 定义日志处理函数
logHandler := func(b []byte, msg string) {
    // b: JSON 格式的结构化日志
    // msg: 格式化好的人类可读字符串
    fmt.Print(msg) 
}

// 创建 gRPC Server 时注入
s := grpc.NewServer(
	grpc.UnaryInterceptor(gm.NewServiceAccessLogger(logHandler)),
)
```

### 2. Metadata Propagation (`PropagateIncomingMetadata`)

将入站请求的 gRPC metadata 自动透传到出站 context 中。

**场景**：
- 在微服务调用链中，保持 `TraceId`、`UserId`、`Language` 等上下文信息的连续传递，确保下游服务能获取到完整的调用链路信息。

**用法**：

```go
s := grpc.NewServer(
	grpc.UnaryInterceptor(gm.PropagateIncomingMetadata),
)
```

### 3. Service Context Injection (`NewInjectServiceContext`)

将当前服务的基本信息注入到 Context 中，用于在发起下游调用时，自动携带当前服务的身份信息。

**注入信息**：
- `AppId`: 当前服务 ID
- `ServiceEndpoint`: 当前服务节点地址
- `ServiceAuthToken`: 服务间认证 Token

**用法**：

```go
import (
    "github.com/fireflycore/go-micro/conf"
    "github.com/fireflycore/go-micro/middleware/grpc"
)

// 假设已有配置对象
var bootstrapConf conf.BootstrapConf 

s := grpc.NewServer(
	grpc.UnaryInterceptor(gm.NewInjectServiceContext(bootstrapConf)),
)
```

## 组合使用

通常建议使用 `grpc.ChainUnaryInterceptor` 组合多个中间件：

```go
s := grpc.NewServer(
    grpc.ChainUnaryInterceptor(
        gm.PropagateIncomingMetadata,           // 1. 先透传元数据
        gm.NewInjectServiceContext(conf),       // 2. 注入当前服务身份
        gm.NewServiceAccessLogger(logHandler),  // 3. 记录访问日志
    ),
)
```
