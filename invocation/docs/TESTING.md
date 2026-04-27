# Invocation Testing

本文档记录 `invocation` 迁移后的最新测试执行结果。

## 测试范围

本轮验证覆盖以下内容：

- 整仓编译与单元测试回归
- `invocation` 包语句覆盖率统计
- 新迁入 `invocation.DNS` 方法的补充单测

本轮重点关注的能力包括：

- `DNS` / `DNSManager` 的默认值补齐、地址构建与目标构建
- `Target` 的地址拼接与派生字符串缓存
- `ConnectionManager` 的连接缓存、关闭与默认拨号逻辑
- `UnaryInvoker` 的 metadata 透传、超时控制与默认调用路径
- `RemoteServiceCaller` 与 `RemoteServiceManaged` 的装配与调用路径

## 测试环境

- 操作系统：`darwin`
- 架构：`arm64`
- CPU：`Apple M2`
- Go 版本：`go1.25.1`
- 模块路径：`github.com/fireflycore/go-micro`
- 目标包：`github.com/fireflycore/go-micro/invocation`

## 执行命令

### 整仓回归

```bash
go test ./...
```

### 包覆盖率

```bash
go test ./invocation -coverprofile=/tmp/invocation.cover.out
go tool cover -func=/tmp/invocation.cover.out
```

## 测试结果

### 整仓回归结果

```text
?       github.com/fireflycore/go-micro/config                  [no test files]
?       github.com/fireflycore/go-micro/config/bootstrap        [no test files]
?       github.com/fireflycore/go-micro/constant                [no test files]
ok      github.com/fireflycore/go-micro/invocation              0.610s
?       github.com/fireflycore/go-micro/kernel                  [no test files]
?       github.com/fireflycore/go-micro/logger                  [no test files]
ok      github.com/fireflycore/go-micro/middleware/grpc         (cached)
?       github.com/fireflycore/go-micro/middleware/http         [no test files]
ok      github.com/fireflycore/go-micro/service                 (cached)
?       github.com/fireflycore/go-micro/sys                     [no test files]
?       github.com/fireflycore/go-micro/telemetry               [no test files]
```

结论：

- `go test ./...` 全部通过
- 删除 `service/dns.go` 后，整仓未出现编译回归
- `middleware/grpc` 与 `service` 包未受本次 DNS 模型迁移影响

### 覆盖率结果

执行结果：

```text
ok      github.com/fireflycore/go-micro/invocation      0.514s  coverage: 91.2% of statements
total:                                                  (statements)   91.2%
```

总覆盖率：

- `91.2% of statements`

新增补测结果：

- `invocation.DNS.Normalize`：`90.0%`
- `invocation.DNS.Build`：`100.0%`
- `invocation.DNS.BuildAddress`：`100.0%`

主要函数覆盖率如下：

| 文件 | 函数 | 覆盖率 |
| --- | --- | --- |
| `caller.go` | `NewRemoteServiceCaller` | `100.0%` |
| `caller.go` | `Invoke` | `100.0%` |
| `dns.go` | `DNS.Normalize` | `90.0%` |
| `dns.go` | `DNS.Build` | `100.0%` |
| `dns.go` | `DNS.BuildAddress` | `100.0%` |
| `dns.go` | `validateDNS` | `100.0%` |
| `dns.go` | `effectivePort` | `100.0%` |
| `dns.go` | `normalize` | `100.0%` |
| `dns.go` | `NewDNSManager` | `100.0%` |
| `dns.go` | `configOrDefault` | `100.0%` |
| `dns.go` | `Config` | `100.0%` |
| `dns.go` | `(*DNSManager).Normalize` | `100.0%` |
| `dns.go` | `(*DNSManager).Build` | `84.6%` |
| `invoker.go` | `defaultUnaryInvokeFunc` | `100.0%` |
| `invoker.go` | `NewUnaryInvoker` | `100.0%` |
| `invoker.go` | `Invoke` | `100.0%` |
| `invoker.go` | `resolveOutgoingMetadata` | `100.0%` |
| `invoker.go` | `NewOutgoingCallContext` | `100.0%` |
| `invoker.go` | `normalizeInvokeTimeout` | `100.0%` |
| `invoker.go` | `newOutgoingCallContextWithOwnedMetadata` | `100.0%` |
| `manager.go` | `normalize` | `80.0%` |
| `manager.go` | `NewConnectionManager` | `66.7%` |
| `manager.go` | `DefaultDialFunc` | `100.0%` |
| `manager.go` | `Dial` | `74.3%` |
| `manager.go` | `Close` | `78.6%` |
| `service.go` | `NewRemoteServiceManaged` | `100.0%` |
| `service.go` | `DNS` | `100.0%` |
| `service.go` | `lookup` | `100.0%` |
| `service.go` | `Caller` | `100.0%` |
| `service.go` | `Invoke` | `83.3%` |
| `target.go` | `Validate` | `100.0%` |
| `target.go` | `Address` | `100.0%` |
| `target.go` | `GRPCTarget` | `100.0%` |
| `target.go` | `cacheDerivedStrings` | `100.0%` |

覆盖率解读：

- `invocation.DNS` 的迁入方法已经纳入单测保护
- `Invoker`、`Target`、`ServiceManaged` 主流程覆盖完整
- 剩余未打满部分仍主要集中在 `ConnectionManager` 的并发与关闭竞争分支

## 当前结论

- 迁移后的整仓回归通过
- `invocation` 包覆盖率维持在 `91.2%`
- 新增 `DNS` 模型方法已纳入测试基线
- 当前结果可以作为发布前的回归与性能对照基线
