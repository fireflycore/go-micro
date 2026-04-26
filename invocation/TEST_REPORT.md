# Invocation Test Report

本文档记录 `invocation` 包当前版本的测试、覆盖率、基准测试与性能对比结果。

测试与基准数据均来自当前仓库本地实测，便于后续回归、评审与性能复盘。

## 1. 测试范围

本次报告覆盖以下内容：

- 单元测试
- 语句覆盖率统计
- 基准测试
- 优化前后性能对比

涉及的核心能力包括：

- `DNSManager` 的默认值补齐、校验与目标构建
- `Target` 的地址拼接与派生字符串缓存
- `ConnectionManager` 的连接缓存、关闭与默认拨号逻辑
- `UnaryInvoker` 的 metadata 透传、超时控制与默认调用路径
- `RemoteServiceCaller` 与 `RemoteServiceManaged` 的装配与调用路径

## 2. 测试环境

- 操作系统：`darwin`
- 架构：`arm64`
- CPU：`Apple M2`
- Go 版本：`go1.25.1`
- 包路径：`github.com/fireflycore/go-micro/invocation`

## 3. 执行命令

### 3.1 单元测试与覆盖率

```bash
go test ./invocation -coverprofile=/tmp/invocation.cover.out
go tool cover -func=/tmp/invocation.cover.out
```

### 3.2 基准测试

```bash
go test ./invocation -run '^$' -bench 'Benchmark(DNSManagerBuild|ConnectionManagerDialCachedParallel|UnaryInvokerInvoke|RemoteServiceManagedInvoke|TargetGRPCTarget)$' -benchmem -count=5
```

## 4. 测试结果

### 4.1 单元测试结果

执行结果：

```text
ok  	github.com/fireflycore/go-micro/invocation	(cached)	coverage: 91.2% of statements
```

结论：

- 当前 `invocation` 包单元测试全部通过
- 当前未发现新增编译错误或诊断错误
- 当前覆盖率已覆盖主要主流程、默认分支和关键错误分支

### 4.2 覆盖率结果

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

## 5. 基准测试设计

基准测试文件：

- `optimization_benchmark_test.go`

当前基准不是仅测试“当前实现有多快”，还保留了 `baseline_old` 与 `optimized` 两组路径：

- `baseline_old`：使用测试辅助函数模拟优化前关键逻辑
- `optimized`：使用当前优化后的正式实现

这样可以在同一台机器、同一套输入与同一份代码上下文内，直接比较优化前后的变化。

## 6. 基准测试原始项

本次基准覆盖以下五项：

- `BenchmarkDNSManagerBuild`
- `BenchmarkConnectionManagerDialCachedParallel`
- `BenchmarkUnaryInvokerInvoke`
- `BenchmarkRemoteServiceManagedInvoke`
- `BenchmarkTargetGRPCTarget`

每项执行 `5` 轮，并统计：

- `ns/op`
- `B/op`
- `allocs/op`

## 7. 性能对比汇总

以下对比基于 5 轮结果的平均值整理。

| 基准项 | 优化前 | 优化后 | 变化 |
| --- | --- | --- | --- |
| `DNSManagerBuild` | `351.84 ns/op` | `349.34 ns/op` | `提升 0.7%` |
| `ConnectionManagerDialCachedParallel` | `560.14 ns/op` | `197.42 ns/op` | `提升 64.8%` |
| `UnaryInvokerInvoke` | `952.82 ns/op` | `756.16 ns/op` | `提升 20.6%` |
| `RemoteServiceManagedInvoke` | `377.26 ns/op` | `353.58 ns/op` | `提升 6.3%` |
| `TargetGRPCTarget` | `162.56 ns/op` | `3.419 ns/op` | `提升 97.9%` |

### 7.1 内存分配对比

| 基准项 | 优化前 | 优化后 | 变化 |
| --- | --- | --- | --- |
| `DNSManagerBuild` | `312 B/op, 12 allocs/op` | `312 B/op, 12 allocs/op` | `持平` |
| `ConnectionManagerDialCachedParallel` | `312 B/op, 12 allocs/op` | `312 B/op, 12 allocs/op` | `持平` |
| `UnaryInvokerInvoke` | `1712 B/op, 22 allocs/op` | `1248 B/op, 16 allocs/op` | `B/op -27.1%, allocs/op -27.3%` |
| `RemoteServiceManagedInvoke` | `480 B/op, 8 allocs/op` | `400 B/op, 7 allocs/op` | `B/op -16.7%, allocs/op -12.5%` |
| `TargetGRPCTarget` | `136 B/op, 6 allocs/op` | `0 B/op, 0 allocs/op` | `完全消除分配` |

## 8. 性能结论

### 8.1 收益最大的优化点

1. `ConnectionManager` 并发缓存命中路径

- 通过 `RWMutex + 双检 + 锁外拨号`，显著降低了缓存命中场景下的锁竞争
- 在并发基准中，平均耗时从 `560.14 ns/op` 降到 `197.42 ns/op`

2. `Target` 派生字符串缓存

- `GRPCTarget()` 和 `Address()` 从“每次格式化”改为“构建后缓存”
- 在基准中，平均耗时从 `162.56 ns/op` 降到 `3.419 ns/op`
- 同时将 `136 B/op, 6 allocs/op` 降为 `0 B/op, 0 allocs/op`

3. `UnaryInvoker` 调用主路径收敛

- 去掉热路径上的匿名闭包创建
- 避免重复 `TrimSpace` 与多余上下文处理
- 调用平均耗时下降 `20.6%`
- 内存分配下降约 `27%`

### 8.2 收益中等的优化点

- `RemoteServiceManaged.Invoke()` 直接走内部 `lookup + invoker`
- 避免每次先构造 `RemoteServiceCaller`
- 单次调用路径有稳定但相对温和的收益

### 8.3 收益较小的优化点

- `DNSManager.Build()` 配置收口带来了逻辑收敛和少量性能改善
- 由于当前主要成本仍在字符串拼接与目标构建本身，该项纯耗时收益相对有限
- 但这项改动对配置一致性和实现清晰度仍然有价值

## 9. 当前结论

当前 `invocation` 包在测试与性能两个维度的状态如下：

- 单元测试通过
- 语句覆盖率达到 `91.2%`
- 关键主流程与错误分支已有较完整保护
- 核心优化路径已通过同仓基准获得实测收益
- 连接缓存热路径、调用主路径与 target 字符串生成的性能改善明显

综合判断：

- 当前版本已经具备可交付的测试完备度
- 当前性能优化结果真实可复现
- 当前报告可作为后续继续演进 `invocation` 的基线文档

## 10. 后续建议

如后续继续增强，可优先考虑以下方向：

- 补充 `ConnectionManager` 更细粒度的并发竞争测试
- 将本报告中的关键基准指标纳入 CI 留档
- 当 `invocation` 再次发生热路径调整时，持续复用本报告中的 benchmark 组合作为回归基线
