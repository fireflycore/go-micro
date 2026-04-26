# Invocation Performance Baseline

本文档记录 `invocation` 包当前版本的基准测试设计、性能对比结果与优化结论。

## 基准范围

本次基准覆盖以下内容：

- `DNSManager.Build()` 的目标构建
- `ConnectionManager` 的并发缓存命中路径
- `UnaryInvoker` 的统一调用路径
- `RemoteServiceManaged.Invoke()` 的调用路径
- `Target.GRPCTarget()` 的派生字符串生成

## 测试环境

- 操作系统：`darwin`
- 架构：`arm64`
- CPU：`Apple M2`
- Go 版本：`go1.25.1`
- 包路径：`github.com/fireflycore/go-micro/invocation`

## 执行命令

```bash
go test ./invocation -run '^$' -bench 'Benchmark(DNSManagerBuild|ConnectionManagerDialCachedParallel|UnaryInvokerInvoke|RemoteServiceManagedInvoke|TargetGRPCTarget)$' -benchmem -count=5
```

## 基准设计

基准测试文件：

- `optimization_benchmark_test.go`

当前基准不是只测“当前实现有多快”，还保留了两组路径：

- `baseline_old`：使用测试辅助函数模拟优化前关键逻辑
- `optimized`：使用当前优化后的正式实现

这样可以在同一台机器、同一套输入与同一份代码上下文内，直接比较优化前后的变化。

## 基准项

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

## 性能对比汇总

以下对比基于 5 轮结果的平均值整理。

| 基准项 | 优化前 | 优化后 | 变化 |
| --- | --- | --- | --- |
| `DNSManagerBuild` | `351.84 ns/op` | `349.34 ns/op` | `提升 0.7%` |
| `ConnectionManagerDialCachedParallel` | `560.14 ns/op` | `197.42 ns/op` | `提升 64.8%` |
| `UnaryInvokerInvoke` | `952.82 ns/op` | `756.16 ns/op` | `提升 20.6%` |
| `RemoteServiceManagedInvoke` | `377.26 ns/op` | `353.58 ns/op` | `提升 6.3%` |
| `TargetGRPCTarget` | `162.56 ns/op` | `3.419 ns/op` | `提升 97.9%` |

### 内存分配对比

| 基准项 | 优化前 | 优化后 | 变化 |
| --- | --- | --- | --- |
| `DNSManagerBuild` | `312 B/op, 12 allocs/op` | `312 B/op, 12 allocs/op` | `持平` |
| `ConnectionManagerDialCachedParallel` | `312 B/op, 12 allocs/op` | `312 B/op, 12 allocs/op` | `持平` |
| `UnaryInvokerInvoke` | `1712 B/op, 22 allocs/op` | `1248 B/op, 16 allocs/op` | `B/op -27.1%, allocs/op -27.3%` |
| `RemoteServiceManagedInvoke` | `480 B/op, 8 allocs/op` | `400 B/op, 7 allocs/op` | `B/op -16.7%, allocs/op -12.5%` |
| `TargetGRPCTarget` | `136 B/op, 6 allocs/op` | `0 B/op, 0 allocs/op` | `完全消除分配` |

## 性能结论

### 收益最大的优化点

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

### 收益中等的优化点

- `RemoteServiceManaged.Invoke()` 直接走内部 `lookup + invoker`
- 避免每次先构造 `RemoteServiceCaller`
- 单次调用路径有稳定但相对温和的收益

### 收益较小的优化点

- `DNSManager.Build()` 配置收口带来了逻辑收敛和少量性能改善
- 由于当前主要成本仍在字符串拼接与目标构建本身，该项纯耗时收益相对有限
- 但这项改动对配置一致性和实现清晰度仍然有价值

## 当前结论

- 核心优化路径已通过同仓基准获得实测收益
- 连接缓存热路径、调用主路径与 target 字符串生成的性能改善明显
- 当前性能结果真实可复现
- 当前文档可作为后续继续演进 `invocation` 的性能基线

## 后续建议

- 补充 `ConnectionManager` 更细粒度的并发竞争测试
- 将关键基准指标纳入 CI 留档
- 当 `invocation` 再次发生热路径调整时，继续复用当前 benchmark 组合作为回归基线
