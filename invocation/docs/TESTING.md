# Invocation Testing

本文档记录 `invocation` 包当前版本的测试范围、执行方式与覆盖率结果。

## 测试范围

本次测试覆盖以下内容：

- 单元测试
- 语句覆盖率统计

涉及的核心能力包括：

- `DNSManager` 的默认值补齐、校验与目标构建
- `Target` 的地址拼接与派生字符串缓存
- `ConnectionManager` 的连接缓存、关闭与默认拨号逻辑
- `UnaryInvoker` 的 metadata 透传、超时控制与默认调用路径
- `RemoteServiceCaller` 与 `RemoteServiceManaged` 的装配与调用路径

## 测试环境

- 操作系统：`darwin`
- 架构：`arm64`
- CPU：`Apple M2`
- Go 版本：`go1.25.1`
- 包路径：`github.com/fireflycore/go-micro/invocation`

## 执行命令

### 单元测试与覆盖率

```bash
go test ./invocation -coverprofile=/tmp/invocation.cover.out
go tool cover -func=/tmp/invocation.cover.out
```

## 测试结果

### 单元测试结果

执行结果：

```text
ok  	github.com/fireflycore/go-micro/invocation	(cached)	coverage: 91.2% of statements
```

结论：

- 当前 `invocation` 包单元测试全部通过
- 当前未发现新增编译错误或诊断错误
- 当前覆盖率已覆盖主要主流程、默认分支和关键错误分支

### 覆盖率结果

总覆盖率：

- `91.2% of statements`

主要函数覆盖率如下：

| 文件 | 函数 | 覆盖率 |
| --- | --- | --- |
| `caller.go` | `NewRemoteServiceCaller` | `100.0%` |
| `caller.go` | `Invoke` | `100.0%` |
| `dns.go` | `validateDNS` | `100.0%` |
| `dns.go` | `effectivePort` | `100.0%` |
| `dns.go` | `normalize` | `100.0%` |
| `dns.go` | `NewDNSManager` | `100.0%` |
| `dns.go` | `configOrDefault` | `100.0%` |
| `dns.go` | `Config` | `100.0%` |
| `dns.go` | `Normalize` | `100.0%` |
| `dns.go` | `Build` | `84.6%` |
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

- `DNS`、`Invoker`、`Target`、`ServiceManaged` 主流程已基本覆盖完整
- 剩余主要未打满部分集中在 `ConnectionManager` 的部分并发/关闭竞争分支
- 当前覆盖率水平已经足以支撑该包继续演进与回归验证

## 当前结论

- 单元测试通过
- 语句覆盖率达到 `91.2%`
- 关键主流程与错误分支已有较完整保护
- 当前测试结果可以作为后续回归验证基线
