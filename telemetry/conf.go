package telemetry

// Config 定义了 Telemetry 的配置结构。
// 它可以映射到配置文件（如 JSON/YAML）中的 telemetry 字段。
type Config struct {
	// OTLPEndpoint 是 OpenTelemetry Collector 的地址 (host:port)。
	OTLPEndpoint string
	// Insecure 决定是否使用非安全连接 (HTTP/gRPC without TLS)。
	Insecure bool

	// Traces 开关，控制是否启用链路追踪。
	Traces bool
	// Logs 开关，控制是否启用日志收集。
	Logs bool
	// Metrics 开关，控制是否启用指标监控。
	Metrics bool
}
