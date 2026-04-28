package bootstrap

type BootstrapConfig struct {
	App       AppConfig       `json:"app"`
	Logger    LoggerConfig    `json:"logger"`
	Service   ServiceConfig   `json:"service"`
	Telemetry TelemetryConfig `json:"telemetry"`
}
