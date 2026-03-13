package conf

type TelemetryConf interface {
	GetOtelEndpoint() string
	GetOtelInsecure() bool

	GetOtelTraces() bool
	GetOtelMetrics() bool
	GetOtelLogs() bool
}
