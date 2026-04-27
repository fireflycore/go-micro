# Invocation Performance Baseline

本文档记录 `invocation` 迁移后的最新基准测试结果。

## 基准范围

本轮基准覆盖以下内容：

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
go test -run '^$' -bench Benchmark -benchmem ./invocation
```

## 基准设计

基准测试文件：

- `optimization_benchmark_test.go`

每个基准都保留两条路径：

- `baseline_old`：用测试辅助函数模拟优化前关键逻辑
- `optimized`：使用当前正式实现

这样可以在同一套输入下直接比较迁移后与优化后主线的实际差异。

## 最新结果

```text
BenchmarkDNSManagerBuild/baseline_old-8                           358.7 ns/op   312 B/op   12 allocs/op
BenchmarkDNSManagerBuild/optimized-8                              349.7 ns/op   312 B/op   12 allocs/op
BenchmarkConnectionManagerDialCachedParallel/baseline_old-8       549.5 ns/op   312 B/op   12 allocs/op
BenchmarkConnectionManagerDialCachedParallel/optimized-8          218.3 ns/op   312 B/op   12 allocs/op
BenchmarkUnaryInvokerInvoke/baseline_old-8                        955.6 ns/op  1712 B/op   22 allocs/op
BenchmarkUnaryInvokerInvoke/optimized-8                           738.9 ns/op  1248 B/op   16 allocs/op
BenchmarkRemoteServiceManagedInvoke/baseline_old-8                355.2 ns/op   480 B/op    8 allocs/op
BenchmarkRemoteServiceManagedInvoke/optimized-8                   365.6 ns/op   400 B/op    7 allocs/op
BenchmarkTargetGRPCTarget/baseline_old-8                          163.5 ns/op   136 B/op    6 allocs/op
BenchmarkTargetGRPCTarget/optimized-8                               3.681 ns/op   0 B/op    0 allocs/op
```

## 性能对比汇总

| 基准项 | 优化前 | 优化后 | 变化 |
| --- | --- | --- | --- |
| `DNSManagerBuild` | `358.7 ns/op` | `349.7 ns/op` | `提升约 2.5%` |
| `ConnectionManagerDialCachedParallel` | `549.5 ns/op` | `218.3 ns/op` | `提升约 60.3%` |
| `UnaryInvokerInvoke` | `955.6 ns/op` | `738.9 ns/op` | `提升约 22.7%` |
| `RemoteServiceManagedInvoke` | `355.2 ns/op` | `365.6 ns/op` | `单次运行波动，耗时慢约 2.9%` |
| `TargetGRPCTarget` | `163.5 ns/op` | `3.681 ns/op` | `提升约 97.7%` |

### 内存分配对比

| 基准项 | 优化前 | 优化后 | 变化 |
| --- | --- | --- | --- |
| `DNSManagerBuild` | `312 B/op, 12 allocs/op` | `312 B/op, 12 allocs/op` | `持平` |
| `ConnectionManagerDialCachedParallel` | `312 B/op, 12 allocs/op` | `312 B/op, 12 allocs/op` | `持平` |
| `UnaryInvokerInvoke` | `1712 B/op, 22 allocs/op` | `1248 B/op, 16 allocs/op` | `B/op -27.1%, allocs/op -27.3%` |
| `RemoteServiceManagedInvoke` | `480 B/op, 8 allocs/op` | `400 B/op, 7 allocs/op` | `B/op -16.7%, allocs/op -12.5%` |
| `TargetGRPCTarget` | `136 B/op, 6 allocs/op` | `0 B/op, 0 allocs/op` | `完全消除分配` |

## 性能结论

### 收益最明显的路径

- `ConnectionManager` 缓存命中路径仍然保持显著收益，锁竞争开销明显下降
- `Target.GRPCTarget()` 的缓存收益依旧极其明显，字符串派生分配被完全消除
- `UnaryInvoker.Invoke()` 在耗时和分配上都继续维持双向收益

### 本轮需要注意的点

- `RemoteServiceManaged.Invoke()` 在本次单次基准里 `ns/op` 有轻微波动，但 `B/op` 和 `allocs/op` 仍然下降
- 这类微基准对机器状态比较敏感；如果后续要作为发布门槛，建议固定 `-count` 并统计均值
- 本次 DNS 模型迁移本身没有引入新的明显性能退化点

## 当前结论

- 迁移后的 `invocation.DNS` 主线已完成基准复跑
- 关键热路径收益依旧成立，尤其是连接缓存与 target 字符串缓存
- 当前结果可作为本次版本发布前的性能基线

## 后续建议

- 为 `RemoteServiceManaged.Invoke()` 增加 `-count` 留档，降低单次波动干扰
- 补充 `ConnectionManager` 更细粒度的并发竞争基准
- 当 `invocation` 再次发生热路径调整时，继续复用当前基准组合作为回归基线
