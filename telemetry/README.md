# Telemetry（OpenTelemetry）接入说明

本目录的 [core.go](file:///d:/project/firefly/go-micro/telemetry/core.go) 用于在服务启动时一次性完成 OpenTelemetry（OTel）的初始化，并把 **Traces / Metrics / Logs** 三类信号的 Provider 设置为全局默认，供其它库（例如 `otelgrpc`、`otelzap`）自动复用。

## 1. OTel 在做什么

OTel 的通用工作模型是：

1. 业务代码或三方库产生信号（Trace/Metric/Log）。
2. 信号交给对应 Provider（TracerProvider/MeterProvider/LoggerProvider）。
3. Provider 通过 Processor/Reader/Exporter 把数据导出到后端（Collector、Prometheus、Tempo、Loki 等）。

其中：

- Trace：`TracerProvider` + `SpanProcessor` + `SpanExporter`
- Metric：`MeterProvider` + `Reader`（Prometheus 是 pull reader）
- Log：`LoggerProvider` + `LogProcessor` + `LogExporter`

## 2. telemetry.Setup 做了哪些事

[Setup](file:///d:/project/firefly/go-micro/telemetry/core.go#L32-L142) 分成 5 个关键步骤：

### 2.1 Resource：统一标识“这是谁发出来的数据”

```go
resource.WithAttributes(
  attribute.String("service.name", bootstrapConf.GetAppName()),
  attribute.String("service.version", bootstrapConf.GetAppVersion()),
)
```

这部分是所有信号的共同“身份标签”。在 Grafana/Tempo/Loki/Prometheus 里按服务聚合、按版本过滤，主要依赖这些资源属性。

### 2.2 Propagator：跨进程传播 Trace 上下文

```go
otel.SetTextMapPropagator(
  propagation.NewCompositeTextMapPropagator(
    propagation.TraceContext{},
    propagation.Baggage{},
  ),
)
```

这意味着链路上下文使用 W3C TraceContext（`traceparent`）标准格式传播。

### 2.3 Traces：OTLP/gRPC 导出到 Collector

当 `bootstrapConf.GetOtelTraces()=true`：

1. 创建 OTLP Trace exporter（gRPC）
2. 创建 `sdktrace.TracerProvider`，使用 `WithBatcher` 批量异步导出
3. 设置全局 tracer provider：`otel.SetTracerProvider(...)`

### 2.4 Metrics：Prometheus pull 模式暴露 /metrics

当 `bootstrapConf.GetOtelMetrics()=true`：

1. 创建 Prometheus registry（隔离 default registry）
2. 创建 OTel Prometheus exporter，并注册到 registry
3. 创建 `sdkmetric.MeterProvider`，`WithReader(metricExp)`
4. 设置全局 meter provider：`otel.SetMeterProvider(...)`
5. 暴露 `providers.MetricsHandler`（`promhttp.HandlerFor(reg, ...)`）

Prometheus 会通过 HTTP 拉取 `/metrics`，所以这里 exporter 不是“push”，而是**提供一个可被 scrape 的视图**。

### 2.5 Logs：OTLP/gRPC 导出到 Collector

当 `bootstrapConf.GetOtelLogs()=true`：

1. 创建 OTLP Log exporter（gRPC）
2. 创建 `sdklog.LoggerProvider`，使用 batch processor 异步导出
3. 设置 Logs 全局 provider：`global.SetLoggerProvider(...)`

## 3. telemetry 如何与 zap 协作（otelzap）

`otelzap` 是 zap 的 bridge：它实现了 `zapcore.Core`，zap 写日志时会调用 core 的 `Write`，`otelzap` 会把 zap 的 `Entry/Fields` 转成 OTel `log.Record` 并发给 OTel 的 `LoggerProvider`。

在 go-micro 中，zap 构造在 [logger.NewZapLogger](file:///d:/project/firefly/go-micro/logger/zap.go#L1-L26)：

- Console=true：追加 console core，输出到 stdout
- Remote=true：追加 `otelzap.NewCore(...)`，输出到 OTel

### 3.1 Trace/Span 关联是怎么做到的

`otelzap` 有一个关键约定：如果 zap fields 里包含 `context.Context`，会用它作为 log record 的上下文，从中读取当前 span，从而自动把日志关联到 trace/span。

go-micro 的 [logger.Core] 专门提供了 `WithContextInfo/WithContextWarn/WithContextError`：

```go
func (l *Core) WithContextInfo(ctx context.Context, msg string, fields ...zap.Field) {
  l.Info(msg, append(fields, zap.Any("ctx", ctx))...)
}
```

因此在请求链路内打日志时，只要传入当前 ctx：

```go
log.WithContextInfo(ctx, "something happened", zap.String("k", "v"))
```

`otelzap` 就能把这条日志自动挂到当前 trace 上。

## 4. telemetry 如何与 gRPC 协作（otelgrpc）：指标与追踪

### 4.1 gRPC 指标：StatsHandler

go-micro 提供了 [gm.NewOtelServerStatsHandler]：

```go
grpc.NewServer(
  grpc.StatsHandler(gm.NewOtelServerStatsHandler()),
)
```

原理是：

- gRPC 内部会把每次 RPC 的开始/结束/字节数等事件回调给 `stats.Handler`
- `otelgrpc.NewServerHandler` 在这些回调中使用全局 `MeterProvider` 记录指标
- 这些指标进入 `telemetry.Setup` 创建的 `MeterProvider`，通过 `providers.MetricsHandler` 暴露给 Prometheus scrape

因此，“gRPC 指标是 otelgrpc 采集的”，而 telemetry 做的是“提供 meter provider + 暴露 /metrics 出口”。

### 4.2 gRPC Trace：UnaryServerInterceptor / StreamServerInterceptor

追踪通常由 otelgrpc 的 interceptor 完成（创建 server span、提取/注入传播头等）。你可以在服务中使用：

```go
grpc.NewServer(
  grpc.ChainUnaryInterceptor(
    otelgrpc.UnaryServerInterceptor(),
    gm.NewAccessLogger(log),
    gm.ValidationErrorToInvalidArgument(),
  ),
)
```

建议把 `otelgrpc.UnaryServerInterceptor()` 放在链路最外层，确保后续日志能从 ctx 里拿到 span 做关联。

## 5. 推荐接入模板（最小可运行骨架）

### 5.1 启动时初始化 telemetry

```go
providers, shutdown, err := telemetry.Setup(bootstrapConf)
if err != nil { panic(err) }
defer shutdown(context.Background())
```

### 5.2 暴露 /metrics

```go
mux := http.NewServeMux()
mux.Handle("/metrics", providers.MetricsHandler)
go http.ListenAndServe(":9090", mux)
```

### 5.3 构造 logger（Console + OTel Remote）

```go
zl := logger.NewZapLogger(bootstrapConf)
log := logger.NewLogger(zl)
```

### 5.4 gRPC Server：StatsHandler + Interceptor

```go
s := grpc.NewServer(
  grpc.StatsHandler(gm.NewOtelServerStatsHandler()),
  grpc.ChainUnaryInterceptor(
    otelgrpc.UnaryServerInterceptor(),
    gm.NewAccessLogger(log),
    gm.ValidationErrorToInvalidArgument(),
  ),
)
_ = s
```

两者的职责区别：

- `grpc.StatsHandler(gm.NewOtelServerStatsHandler())`
  - 属于 gRPC 的 `stats.Handler` 回调机制
  - 主要负责 Metrics：采集 RPC 过程中的统计事件（开始/结束/字节数等）并记录为指标，最终通过 `/metrics` 暴露给 Prometheus scrape
- `otelgrpc.UnaryServerInterceptor()`
  - 属于 gRPC 的 Unary 拦截器链
  - 主要负责 Traces：为每个 RPC 创建/续接 server span，把 span 放进 `ctx`，并在结束时记录错误与状态；同时负责上下文传播（解析/注入 W3C TraceContext）

## 6. 常见排查点

- 在 Tempo 看不到 trace：
  - 是否启用了 `bootstrapConf.GetOtelTraces()=true`
  - 是否挂了 `otelgrpc.UnaryServerInterceptor()` 或者其它 instrumentation
  - OTLP endpoint 是否可达
- /metrics 没有 gRPC 指标：
  - 是否启用了 `bootstrapConf.GetOtelMetrics()=true`
  - 是否挂了 `grpc.StatsHandler(gm.NewOtelServerStatsHandler())`
  - Prometheus 是否 scrape 到正确端口/路径
- 日志无法和 trace 关联：
  - 打日志时是否把 `ctx` 传进 `WithContextInfo/WithContextError`
  - 是否确实存在当前 span（是否装了 otelgrpc interceptor）
