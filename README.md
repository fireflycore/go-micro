# Firefly Go Micro

`go-micro` 是 Firefly 微服务框架的 Go 版本核心库，提供了构建微服务所需的基础设施抽象与通用工具。

建议配合 **go-layout**（Firefly 微服务框架的 Go 版本标准项目模板）使用，以获得最佳开发体验。

## 安装

```bash
go get github.com/fireflycore/go-micro
```

## 快速开始

以 gRPC 服务为例，常见用法是把中间件注入到 `grpc.Server`：

```go
import (
	"fmt"

	"github.com/fireflycore/go-micro/middleware"
	"google.golang.org/grpc"
)

s := grpc.NewServer(
	grpc.ChainUnaryInterceptor(
		middleware.PropagateIncomingMetadata,
		middleware.GrpcAccessLogger(func(_ []byte, msg string) {
			fmt.Println(msg)
		}),
	),
)
_ = s
```

## 模块说明

详细文档请参考各子包目录下的 README：

- [registry](./registry/README.md)：服务发现与注册
- [rpc](./rpc/README.md)：RPC 调用封装
- [middleware](./middleware/README.md)：gRPC 中间件
- [logger](./logger/README.md)：日志级别定义
- [constant](./constant/README.md)：通用常量
- [utils](./utils/README.md)：工具函数

## 许可证

[LICENSE](./LICENSE)
