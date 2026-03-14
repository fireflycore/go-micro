package telemetry

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/fireflycore/go-micro/conf"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Providers 聚合了 OpenTelemetry 的三个主要 Provider：
// - TracerProvider: 用于链路追踪
// - MeterProvider: 用于指标监控
// - LoggerProvider: 用于日志记录
type Providers struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	LoggerProvider *sdklog.LoggerProvider

	// MetricsHandler 是 Prometheus 的 HTTP Handler，
	// 用于对外暴露 /metrics 接口供 Prometheus Server 拉取数据。
	MetricsHandler http.Handler
}

// DefaultInitTimeout 是初始化 Telemetry 的默认超时时间。
const DefaultInitTimeout = 3 * time.Second

// NewProviders 创建并初始化 Telemetry Providers。
// 这是一个便捷函数，内部调用 SetupWithContext，使用 DefaultInitTimeout。
//
// 参数:
//   - bootstrapConf: 引导配置，包含 OTel 配置信息
//
// 返回:
//   - *Providers: 包含 Tracer, Meter, Logger Provider
//   - func(context.Context) error: 关闭函数，用于优雅关闭 Providers
//   - error: 初始化错误
func NewProviders(bootstrapConf conf.BootstrapConf) (*Providers, func(context.Context) error, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeout)
	defer cancel()

	return SetupWithContext(ctx, bootstrapConf)
}

// SetupWithContext 使用给定的上下文初始化 Telemetry Providers。
// 它会根据 bootstrapConf 中的配置决定是否启用 Traces, Metrics, Logs。
//
// 主要步骤:
//  1. 创建 Resource (Service Name, Version)
//  2. 设置全局 Propagator (TraceContext, Baggage)
//  3. 初始化 Traces (如果启用)
//  4. 初始化 Metrics (如果启用)
//  5. 初始化 Logs (如果启用)
//
// 返回:
//   - *Providers: 包含 Tracer, Meter, Logger Provider
//   - func(context.Context) error: 关闭函数，用于优雅关闭 Providers
//   - error: 初始化错误
func SetupWithContext(ctx context.Context, bootstrapConf conf.BootstrapConf) (*Providers, func(context.Context) error, error) {
	if bootstrapConf == nil {
		return nil, nil, errors.New("bootstrap conf is nil")
	}

	// 1. 创建 Resource
	res, err := NewResource(ctx, bootstrapConf)
	if err != nil {
		return nil, nil, err
	}

	p := &Providers{}

	// 2. 设置全局 Propagator
	// 使用 W3C Trace Context 和 Baggage 标准
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	otlpEndpoint := bootstrapConf.GetOtelEndpoint()
	insecure := bootstrapConf.GetOtelInsecure()

	// 3. 初始化 Traces
	if bootstrapConf.GetOtelTraces() {
		tp, err := NewTracerProvider(ctx, res, otlpEndpoint, insecure)
		if err != nil {
			return nil, nil, err
		}
		p.TracerProvider = tp
		// 设置全局 TracerProvider
		otel.SetTracerProvider(tp)
	}

	// 4. 初始化 Metrics
	if bootstrapConf.GetOtelMetrics() {
		mp, mh, err := NewMeterProvider(res)
		if err != nil {
			return nil, nil, err
		}
		p.MeterProvider = mp
		p.MetricsHandler = mh
		// 设置全局 MeterProvider
		otel.SetMeterProvider(mp)
	}

	// 5. 初始化 Logs
	if bootstrapConf.GetOtelLogs() {
		lp, err := NewLoggerProvider(ctx, res, otlpEndpoint, insecure)
		if err != nil {
			return nil, nil, err
		}
		p.LoggerProvider = lp
		// 设置全局 LoggerProvider
		global.SetLoggerProvider(lp)
	}

	// 定义关闭函数，按顺序关闭所有 Provider
	shutdown := func(ctx context.Context) error {
		var out error
		if p.LoggerProvider != nil {
			if err := p.LoggerProvider.Shutdown(ctx); err != nil {
				out = errors.Join(out, err)
			}
		}
		if p.MeterProvider != nil {
			if err := p.MeterProvider.Shutdown(ctx); err != nil {
				out = errors.Join(out, err)
			}
		}
		if p.TracerProvider != nil {
			if err := p.TracerProvider.Shutdown(ctx); err != nil {
				out = errors.Join(out, err)
			}
		}
		return out
	}

	return p, shutdown, nil
}

// NewResource 创建 OpenTelemetry Resource。
// Resource 包含描述实体的属性，例如服务名称、版本等。
func NewResource(ctx context.Context, bootstrapConf conf.BootstrapConf) (*resource.Resource, error) {
	return resource.New(
		ctx,
		resource.WithAttributes(
			attribute.String("service.app.name", bootstrapConf.GetAppName()),
			attribute.String("service.app.version", bootstrapConf.GetAppVersion()),
			attribute.String("service.app.id", bootstrapConf.GetAppId()),
		),
	)
}
