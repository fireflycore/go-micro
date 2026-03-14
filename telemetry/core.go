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
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
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
// 初始化过程使用 DefaultInitTimeout 作为默认超时控制。
//
// 参数:
//   - bootstrapConf: 引导配置，包含 OTel 配置信息
//
// 返回:
//   - *Providers: 包含 Tracer, Meter, Logger Provider
//   - error: 初始化错误
func NewProviders(bootstrapConf conf.BootstrapConf) (*Providers, error) {
	if bootstrapConf == nil {
		return nil, errors.New("bootstrap conf is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeout)
	defer cancel()

	p := &Providers{}

	// 1. 创建 Resource
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(bootstrapConf.GetAppName()),
			semconv.ServiceVersion(bootstrapConf.GetAppVersion()),
			semconv.ServiceNamespace(bootstrapConf.GetServiceNamespace()),
			semconv.ServiceInstanceID(bootstrapConf.GetServiceInstanceId()),
			attribute.String("service.id", bootstrapConf.GetAppId()),
		),
	)
	if err != nil {
		return nil, err
	}

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
			return nil, err
		}
		p.TracerProvider = tp
		// 设置全局 TracerProvider
		otel.SetTracerProvider(tp)
	}

	// 4. 初始化 Metrics
	if bootstrapConf.GetOtelMetrics() {
		mp, mh, err := NewMeterProvider(res)
		if err != nil {
			return nil, err
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
			return nil, err
		}
		p.LoggerProvider = lp
		// 设置全局 LoggerProvider
		global.SetLoggerProvider(lp)
	}

	return p, nil
}

func (p *Providers) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeout)
	defer cancel()

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
