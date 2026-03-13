package telemetry

// Conf 定义了 Telemetry 的配置结构。
// 它可以映射到配置文件（如 JSON/YAML）中的 telemetry 字段。
type Conf struct {
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

// GetOtelEndpoint 获取 OTLP Endpoint。
func (c *Conf) GetOtelEndpoint() string {
	if c == nil {
		return ""
	}
	return c.OTLPEndpoint
}

// GetOTLPEndpoint 获取 OTLP Endpoint (别名)。
func (c *Conf) GetOTLPEndpoint() string {
	if c == nil {
		return ""
	}
	return c.OTLPEndpoint
}

// GetOtelInsecure 获取是否使用非安全连接。
func (c *Conf) GetOtelInsecure() bool {
	if c == nil {
		return false
	}
	return c.Insecure
}

// GetOtelTraces 获取 Traces 开关状态。
func (c *Conf) GetOtelTraces() bool {
	if c == nil {
		return false
	}
	return c.Traces
}

// GetOtelMetrics 获取 Metrics 开关状态。
func (c *Conf) GetOtelMetrics() bool {
	if c == nil {
		return false
	}
	return c.Metrics
}

// GetOtelLogs 获取 Logs 开关状态。
func (c *Conf) GetOtelLogs() bool {
	if c == nil {
		return false
	}
	return c.Logs
}
