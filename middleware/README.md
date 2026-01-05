# Middleware

`middleware` 包提供了 gRPC 服务的通用中间件（Interceptor）。

## 功能列表

### 1. Access Logger (`GrpcAccessLogger`)

提供详细的 gRPC 访问日志记录功能。

**特性**：
- 自动记录请求/响应耗时。
- 自动提取 gRPC metadata 中的客户端信息（IP、System、Client、Version 等）。
- 自动关联 `TraceId`（若缺失则自动生成）。
- 记录 Request/Response JSON 载荷。

**用法**：

```go
import (
	"fmt"

	"github.com/fireflycore/go-micro/middleware"
	"google.golang.org/grpc"
)

// 创建 gRPC Server 时注入
s := grpc.NewServer(
	grpc.UnaryInterceptor(middleware.GrpcAccessLogger(func(b []byte, msg string) {
		// 自定义日志输出逻辑，例如写入文件或 stdout
		fmt.Println(string(b))
	})),
)
```

### 2. Metadata Propagation (`PropagateIncomingMetadata`)

将入站请求的 gRPC metadata 自动透传到出站 context 中。

**场景**：
- 在微服务调用链中，保持 `TraceId`、`UserId` 等上下文信息的连续传递。

**用法**：

```go
import (
	"github.com/fireflycore/go-micro/middleware"
	"google.golang.org/grpc"
)

s := grpc.NewServer(
	grpc.UnaryInterceptor(middleware.PropagateIncomingMetadata),
)
```
