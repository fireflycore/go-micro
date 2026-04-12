# UserContext 使用指南

## 概述

`invocation` 包提供了高效的用户上下文管理机制，通过"解析一次，到处使用"的模式，避免重复解析 metadata，提升性能。

## 核心 API

### 1. ParseUserContextMeta

从 gRPC metadata 解析用户上下文：

```go
func ParseUserContextMeta(md metadata.MD) (*UserContextMeta, error)
```

### 2. WithUserContext

将 UserContextMeta 存入 context：

```go
func WithUserContext(ctx context.Context, meta *UserContextMeta) context.Context
```

### 3. UserContextFromContext

从 context 获取 UserContextMeta（安全版本）：

```go
func UserContextFromContext(ctx context.Context) (*UserContextMeta, bool)
```

### 4. MustUserContextFromContext

从 context 获取 UserContextMeta（panic 版本）：

```go
func MustUserContextFromContext(ctx context.Context) *UserContextMeta
```

## 完整使用示例

### 步骤 1：创建 Server Interceptor

在 gRPC server 启动时，添加统一的 interceptor 来解析用户上下文：

```go
package middleware

import (
    "context"
    "log"
    
    "github.com/fireflycore/go-micro/invocation"
    "google.golang.org/grpc"
    "google.golang.org/grpc/metadata"
)

// UserContextInterceptor 解析用户上下文并存入 context
func UserContextInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        // 从 incoming metadata 获取
        md, ok := metadata.FromIncomingContext(ctx)
        if !ok {
            log.Println("no metadata in context")
            return handler(ctx, req)
        }
        
        // 解析用户上下文
        userMeta, err := invocation.ParseUserContextMeta(md)
        if err != nil {
            log.Printf("failed to parse user context: %v", err)
            return handler(ctx, req)
        }
        
        // 存入 context（关键步骤！）
        ctx = invocation.WithUserContext(ctx, userMeta)
        
        // 继续处理请求
        return handler(ctx, req)
    }
}
```

### 步骤 2：注册 Interceptor

在 server 启动时注册：

```go
package main

import (
    "github.com/fireflycore/go-micro/middleware"
    "google.golang.org/grpc"
)

func main() {
    server := grpc.NewServer(
        grpc.ChainUnaryInterceptor(
            middleware.UserContextInterceptor(),  // 添加用户上下文 interceptor
            // 其他 interceptor...
        ),
    )
    
    // 注册服务...
    // 启动 server...
}
```

### 步骤 3：在 Handler 中使用

#### 方式 1：安全获取（推荐）

```go
package service

import (
    "context"
    "log"
    
    "github.com/fireflycore/go-micro/invocation"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

type UserService struct {
    // ...
}

func (s *UserService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
    // 从 context 获取用户上下文（安全版本）
    userMeta, ok := invocation.UserContextFromContext(ctx)
    if !ok {
        return nil, status.Error(codes.Unauthenticated, "user context not found")
    }
    
    // 使用用户上下文
    log.Printf("user request: user_id=%s, tenant_id=%s", userMeta.UserId, userMeta.TenantId)
    
    // 权限检查
    if userMeta.UserId != req.UserId {
        return nil, status.Error(codes.PermissionDenied, "cannot access other user's data")
    }
    
    // 业务逻辑...
    return &pb.GetUserResponse{
        User: &pb.User{
            Id:       userMeta.UserId,
            TenantId: userMeta.TenantId,
        },
    }, nil
}
```

#### 方式 2：Must 版本（确定存在时使用）

```go
func (s *UserService) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
    // 如果确定 interceptor 一定会设置 user context，可以用 Must 版本
    userMeta := invocation.MustUserContextFromContext(ctx)
    
    log.Printf("update user: user_id=%s", userMeta.UserId)
    
    // 业务逻辑...
    return &pb.UpdateUserResponse{}, nil
}
```

### 步骤 4：在中间件中使用

```go
package middleware

import (
    "context"
    "errors"
    
    "github.com/fireflycore/go-micro/invocation"
)

// AuthzMiddleware 权限检查中间件
func AuthzMiddleware(requiredRole string) func(context.Context, any) error {
    return func(ctx context.Context, req any) error {
        // 直接从 context 获取，无需重复解析
        userMeta, ok := invocation.UserContextFromContext(ctx)
        if !ok {
            return errors.New("unauthorized: user context not found")
        }
        
        // 检查角色权限
        hasRole := false
        for _, role := range userMeta.RoleIds {
            if role == requiredRole {
                hasRole = true
                break
            }
        }
        
        if !hasRole {
            return errors.New("forbidden: insufficient permissions")
        }
        
        return nil
    }
}
```

### 步骤 5：在日志中使用

```go
package logger

import (
    "context"
    
    "github.com/fireflycore/go-micro/invocation"
    "go.uber.org/zap"
)

func LogWithUserContext(ctx context.Context, logger *zap.Logger, msg string) {
    // 从 context 获取用户信息
    userMeta, ok := invocation.UserContextFromContext(ctx)
    if !ok {
        logger.Info(msg)
        return
    }
    
    // 带上用户信息记录日志
    logger.Info(msg,
        zap.String("user_id", userMeta.UserId),
        zap.String("tenant_id", userMeta.TenantId),
        zap.String("session", userMeta.Session),
        zap.String("client_ip", userMeta.ClientIp),
    )
}
```

## 性能对比

### 重构前（不推荐）

每次使用都需要解析：

```go
func Handler1(ctx context.Context) error {
    md, _ := metadata.FromIncomingContext(ctx)
    userMeta, _ := invocation.ParseUserContextMeta(md)  // 第1次解析
    // ...
}

func Handler2(ctx context.Context) error {
    md, _ := metadata.FromIncomingContext(ctx)
    userMeta, _ := invocation.ParseUserContextMeta(md)  // 第2次解析（重复！）
    // ...
}
```

**问题：**
- ❌ 重复解析，浪费 CPU
- ❌ 代码冗余
- ❌ 容易出错

### 重构后（推荐）

解析一次，到处使用：

```go
// Interceptor 中解析一次
func Interceptor(ctx context.Context, ...) {
    md, _ := metadata.FromIncomingContext(ctx)
    userMeta, _ := invocation.ParseUserContextMeta(md)  // 只解析一次
    ctx = invocation.WithUserContext(ctx, userMeta)     // 存入 context
    // ...
}

// Handler 中直接获取
func Handler1(ctx context.Context) error {
    userMeta, _ := invocation.UserContextFromContext(ctx)  // O(1) 获取
    // ...
}

func Handler2(ctx context.Context) error {
    userMeta, _ := invocation.UserContextFromContext(ctx)  // O(1) 获取
    // ...
}
```

**优势：**
- ✅ 只解析一次，性能更好
- ✅ 代码简洁
- ✅ 不易出错

## 测试示例

### 单元测试

```go
package service

import (
    "context"
    "testing"
    
    "github.com/fireflycore/go-micro/invocation"
)

func TestGetUser(t *testing.T) {
    // 准备测试数据
    userMeta := &invocation.UserContextMeta{
        UserId:   "user-123",
        TenantId: "tenant-456",
        Session:  "session-789",
        ClientIp: "192.168.1.1",
        AppId:    "app-1",
        RoleIds:  []string{"admin"},
        OrgIds:   []string{"org-1"},
    }
    
    // 创建带用户上下文的 context
    ctx := context.Background()
    ctx = invocation.WithUserContext(ctx, userMeta)
    
    // 调用被测试的函数
    service := &UserService{}
    resp, err := service.GetUser(ctx, &pb.GetUserRequest{
        UserId: "user-123",
    })
    
    // 验证结果
    if err != nil {
        t.Fatalf("GetUser() error = %v", err)
    }
    if resp.User.Id != "user-123" {
        t.Errorf("User.Id = %v, want user-123", resp.User.Id)
    }
}
```

### 集成测试

```go
package integration

import (
    "context"
    "testing"
    
    "github.com/fireflycore/go-micro/constant"
    "github.com/fireflycore/go-micro/invocation"
    "google.golang.org/grpc/metadata"
)

func TestUserContextFlow(t *testing.T) {
    // 模拟 gRPC incoming metadata
    md := metadata.MD{
        constant.Session:  []string{"session-123"},
        constant.ClientIp: []string{"192.168.1.1"},
        constant.UserId:   []string{"user-1"},
        constant.AppId:    []string{"app-1"},
        constant.TenantId: []string{"tenant-1"},
        constant.RoleIds:  []string{"admin"},
        constant.OrgIds:   []string{"org-1"},
    }
    
    ctx := metadata.NewIncomingContext(context.Background(), md)
    
    // 模拟 interceptor 的行为
    incomingMd, _ := metadata.FromIncomingContext(ctx)
    userMeta, err := invocation.ParseUserContextMeta(incomingMd)
    if err != nil {
        t.Fatalf("ParseUserContextMeta() error = %v", err)
    }
    
    ctx = invocation.WithUserContext(ctx, userMeta)
    
    // 模拟 handler 的行为
    gotMeta, ok := invocation.UserContextFromContext(ctx)
    if !ok {
        t.Fatal("UserContextFromContext() failed")
    }
    
    if gotMeta.UserId != "user-1" {
        t.Errorf("UserId = %v, want user-1", gotMeta.UserId)
    }
}
```

## 常见问题

### Q1: 为什么要用 context.Value 而不是直接传参？

**A:** 使用 context.Value 的优势：
1. 符合 Go 标准实践
2. 不需要修改函数签名
3. 可以在调用链中自动传递
4. 与 gRPC interceptor 配合良好

### Q2: UserContextFromContext 和 MustUserContextFromContext 有什么区别？

**A:**
- `UserContextFromContext`: 安全版本，返回 `(meta, ok)`，不存在时返回 `(nil, false)`
- `MustUserContextFromContext`: panic 版本，不存在时会 panic

推荐在大多数场景使用 `UserContextFromContext`，只在确定一定存在时使用 `MustUserContextFromContext`。

### Q3: 如果 interceptor 没有设置 user context 怎么办？

**A:** 在 handler 中使用 `UserContextFromContext` 检查：

```go
userMeta, ok := invocation.UserContextFromContext(ctx)
if !ok {
    return status.Error(codes.Unauthenticated, "user context not found")
}
```

### Q4: 可以在 context 中存储其他信息吗？

**A:** 可以！你可以定义自己的 context key：

```go
type contextKey string

const myDataKey contextKey = "my_data"

func WithMyData(ctx context.Context, data *MyData) context.Context {
    return context.WithValue(ctx, myDataKey, data)
}

func MyDataFromContext(ctx context.Context) (*MyData, bool) {
    data, ok := ctx.Value(myDataKey).(*MyData)
    return data, ok
}
```

## 最佳实践

1. ✅ **在 interceptor 中解析一次**：避免在每个 handler 中重复解析
2. ✅ **使用 UserContextFromContext**：优先使用安全版本，检查返回值
3. ✅ **记录日志时带上用户信息**：便于问题排查
4. ✅ **在单元测试中使用 WithUserContext**：方便构造测试数据
5. ❌ **不要在 handler 中调用 ParseUserContextMeta**：应该从 context 获取

## 总结

通过"解析一次，到处使用"的模式：
- ✅ 提升性能（避免重复解析）
- ✅ 简化代码（无需重复调用 ParseUserContextMeta）
- ✅ 减少错误（统一在 interceptor 中处理）
- ✅ 易于测试（使用 WithUserContext 构造测试数据）
