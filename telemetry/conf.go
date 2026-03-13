package telemetry

type Conf struct {
	OTLPEndpoint string
	Insecure     bool

	Traces  bool
	Logs    bool
	Metrics bool
}

func (c *Conf) GetOtelEndpoint() string {
	if c == nil {
		return ""
	}
	return c.OTLPEndpoint
}

func (c *Conf) GetOTLPEndpoint() string {
	if c == nil {
		return ""
	}
	return c.OTLPEndpoint
}

func (c *Conf) GetOtelInsecure() bool {
	if c == nil {
		return false
	}
	return c.Insecure
}

func (c *Conf) GetOtelTraces() bool {
	if c == nil {
		return false
	}
	return c.Traces
}

func (c *Conf) GetOtelMetrics() bool {
	if c == nil {
		return false
	}
	return c.Metrics
}

func (c *Conf) GetOtelLogs() bool {
	if c == nil {
		return false
	}
	return c.Logs
}
