package telemetry

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/fireflycore/go-micro/conf"
	promreg "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Providers struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	LoggerProvider *sdklog.LoggerProvider

	MetricsHandler http.Handler
}

const DefaultInitTimeout = 3 * time.Second

func NewProviders(bootstrapConf conf.BootstrapConf) (*Providers, func(context.Context) error, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return SetupWithContext(ctx, bootstrapConf)
}

func SetupWithContext(ctx context.Context, bootstrapConf conf.BootstrapConf) (*Providers, func(context.Context) error, error) {
	if bootstrapConf == nil {
		return nil, nil, errors.New("bootstrap conf is nil")
	}

	res, err := newResource(ctx, bootstrapConf)
	if err != nil {
		return nil, nil, err
	}

	p := &Providers{}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	otlpEndpoint := bootstrapConf.GetOtelEndpoint()
	insecure := bootstrapConf.GetOtelInsecure()

	if bootstrapConf.GetOtelTraces() {
		tp, te := setupTraces(ctx, res, otlpEndpoint, insecure)
		if te != nil {
			return nil, nil, te
		}
		p.TracerProvider = tp
		otel.SetTracerProvider(tp)
	}

	if bootstrapConf.GetOtelMetrics() {
		mp, mh, me := setupMetrics(res)
		if me != nil {
			return nil, nil, me
		}
		p.MeterProvider = mp
		p.MetricsHandler = mh
		otel.SetMeterProvider(mp)
	}

	if bootstrapConf.GetOtelLogs() {
		lp, le := setupLogs(ctx, res, otlpEndpoint, insecure)
		if le != nil {
			return nil, nil, le
		}
		p.LoggerProvider = lp
		global.SetLoggerProvider(lp)
	}

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

func newResource(ctx context.Context, bootstrapConf conf.BootstrapConf) (*resource.Resource, error) {
	return resource.New(
		ctx,
		resource.WithAttributes(
			attribute.String("service.name", bootstrapConf.GetAppName()),
			attribute.String("service.version", bootstrapConf.GetAppVersion()),
		),
	)
}

func setupTraces(ctx context.Context, res *resource.Resource, otlpEndpoint string, insecure bool) (*sdktrace.TracerProvider, error) {
	expOpts := make([]otlptracegrpc.Option, 0, 2)
	if otlpEndpoint != "" {
		expOpts = append(expOpts, otlptracegrpc.WithEndpoint(otlpEndpoint))
	}
	if insecure {
		expOpts = append(expOpts, otlptracegrpc.WithInsecure())
	}

	traceExp, err := otlptracegrpc.New(ctx, expOpts...)
	if err != nil {
		return nil, err
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExp),
	), nil
}

func setupMetrics(res *resource.Resource) (*sdkmetric.MeterProvider, http.Handler, error) {
	reg := promreg.NewRegistry()

	metricExp, err := otelprom.New(
		otelprom.WithRegisterer(reg),
	)
	if err != nil {
		return nil, nil, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(metricExp),
	)

	return mp, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}), nil
}

func setupLogs(ctx context.Context, res *resource.Resource, otlpEndpoint string, insecure bool) (*sdklog.LoggerProvider, error) {
	expOpts := make([]otlploggrpc.Option, 0, 2)
	if otlpEndpoint != "" {
		expOpts = append(expOpts, otlploggrpc.WithEndpoint(otlpEndpoint))
	}
	if insecure {
		expOpts = append(expOpts, otlploggrpc.WithInsecure())
	}

	logExp, err := otlploggrpc.New(ctx, expOpts...)
	if err != nil {
		return nil, err
	}

	return sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
	), nil
}
