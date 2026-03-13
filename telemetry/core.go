package telemetry

import (
	"context"
	"errors"
	"net/http"

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

func Setup(ctx context.Context, cfg *Conf) (*Providers, func(context.Context) error, error) {
	if cfg.ServiceName == "" {
		return nil, nil, errors.New("service name is required")
	}

	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	p := &Providers{}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if cfg.Traces {
		expOpts := make([]otlptracegrpc.Option, 0, 2)
		if cfg.OTLPEndpoint != "" {
			expOpts = append(expOpts, otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint))
		}
		if cfg.Insecure {
			expOpts = append(expOpts, otlptracegrpc.WithInsecure())
		}

		traceExp, te := otlptracegrpc.New(ctx, expOpts...)
		if te != nil {
			return nil, nil, te
		}

		p.TracerProvider = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithBatcher(traceExp),
		)
		otel.SetTracerProvider(p.TracerProvider)
	}

	if cfg.Metrics {
		reg := promreg.NewRegistry()

		metricExp, me := otelprom.New(
			otelprom.WithRegisterer(reg),
		)
		if me != nil {
			return nil, nil, me
		}

		p.MeterProvider = sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
			sdkmetric.WithReader(metricExp),
		)
		otel.SetMeterProvider(p.MeterProvider)
		p.MetricsHandler = promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	}

	if cfg.Logs {
		expOpts := make([]otlploggrpc.Option, 0, 2)
		if cfg.OTLPEndpoint != "" {
			expOpts = append(expOpts, otlploggrpc.WithEndpoint(cfg.OTLPEndpoint))
		}
		if cfg.Insecure {
			expOpts = append(expOpts, otlploggrpc.WithInsecure())
		}

		logExp, le := otlploggrpc.New(ctx, expOpts...)
		if le != nil {
			return nil, nil, le
		}

		p.LoggerProvider = sdklog.NewLoggerProvider(
			sdklog.WithResource(res),
			sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		)
		global.SetLoggerProvider(p.LoggerProvider)
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
