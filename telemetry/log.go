package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
)

// NewLoggerProvider 初始化 LoggerProvider 并配置 OTLP gRPC 导出器。
//
// 参数:
//   - ctx: 上下文
//   - res: 资源属性（包含服务名、版本等）
//   - otlpEndpoint: OTLP 收集器地址
//   - insecure: 是否使用非安全连接
//
// 返回:
//   - *sdklog.LoggerProvider: 配置好的 LoggerProvider
//   - error: 初始化过程中的错误
func NewLoggerProvider(ctx context.Context, res *resource.Resource, otlpEndpoint string, insecure bool) (*sdklog.LoggerProvider, error) {
	expOpts := make([]otlploggrpc.Option, 0, 2)
	if otlpEndpoint != "" {
		expOpts = append(expOpts, otlploggrpc.WithEndpoint(otlpEndpoint))
	}
	if insecure {
		expOpts = append(expOpts, otlploggrpc.WithInsecure())
	}

	// 创建 OTLP Log Exporter
	logExp, err := otlploggrpc.New(ctx, expOpts...)
	if err != nil {
		return nil, err
	}

	// 创建 LoggerProvider，使用 BatchProcessor 批量处理日志
	return sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
	), nil
}
