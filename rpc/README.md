# RPC

`rpc` 包提供了标准化的 RPC 调用封装与响应处理工具。

## 设计理念

本包旨在统一处理微服务间的响应格式，遵循 **Code/Message/Data** 模式：
- **Code**: 业务状态码（200 表示成功）。
- **Message**: 业务提示信息。
- **Data**: 实际业务数据。

## 核心功能

### `WithRemoteInvoke`

泛型函数，用于发起远程调用并自动处理响应解包。

**处理逻辑**：
1. **网络错误**：直接返回 error。
2. **Nil 检查**：防御性处理“带类型的 nil”。
3. **业务错误**：若 `Code != 200`，提取 `Message` 并封装为 error 返回。
4. **成功**：仅返回 `Data` 部分。

## 使用示例

假设 Proto 定义如下：
```protobuf
message GetUserResponse {
    uint32 code = 1;
    string message = 2;
    User data = 3;
}
```

调用代码：
```go
import "github.com/fireflycore/go-micro/rpc"

// 泛型参数 T 为业务数据类型 (User)
// 泛型参数 R 为响应类型 (GetUserResponse)
user, err := rpc.WithRemoteInvoke[pb.User](func() (rpc.RemoteResponse[pb.User], error) {
    return client.GetUser(ctx, &pb.GetUserRequest{Id: 1})
})

if err != nil {
    // 处理网络错误或业务错误（Code!=200）
    return err
}

// 直接使用 user 对象
fmt.Println(user.Name)
```
