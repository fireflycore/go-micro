package config

type TelemetryConfig interface {
	GetOtelEndpoint() string
	GetOtelInsecure() bool

	GetOtelTraces() bool
	GetOtelMetrics() bool
	GetOtelLogs() bool
}
