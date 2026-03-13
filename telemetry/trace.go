package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// NewTracerProvider 初始化 TracerProvider 并配置 OTLP gRPC 导出器。
//
// 参数:
//   - ctx: 上下文
//   - res: 资源属性（包含服务名、版本等）
//   - otlpEndpoint: OTLP 收集器地址（例如 "localhost:4317"）
//   - insecure: 是否使用非安全连接（HTTP/gRPC without TLS）
//
// 返回:
//   - *sdktrace.TracerProvider: 配置好的 TracerProvider
//   - error: 初始化过程中的错误
func NewTracerProvider(ctx context.Context, res *resource.Resource, otlpEndpoint string, insecure bool) (*sdktrace.TracerProvider, error) {
	expOpts := make([]otlptracegrpc.Option, 0, 2)
	if otlpEndpoint != "" {
		expOpts = append(expOpts, otlptracegrpc.WithEndpoint(otlpEndpoint))
	}
	if insecure {
		expOpts = append(expOpts, otlptracegrpc.WithInsecure())
	}

	// 创建 OTLP Trace Exporter
	traceExp, err := otlptracegrpc.New(ctx, expOpts...)
	if err != nil {
		return nil, err
	}

	// 创建 TracerProvider，使用 Batcher 批量导出 Span
	return sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExp),
	), nil
}
