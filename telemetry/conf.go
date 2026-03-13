package telemetry

type Conf struct {
	OTLPEndpoint string
	Insecure     bool

	Traces  bool
	Logs    bool
	Metrics bool
}

func (c *Conf) GetOTLPEndpoint() string {
	return c.OTLPEndpoint
}

func (c *Conf) GetOtelInsecure() bool {
	return c.Insecure
}

func (c *Conf) GetOtelTraces() bool {
	return c.Traces
}

func (c *Conf) GetOtelMetrics() bool {
	return c.Metrics
}

func (c *Conf) GetOtelLogs() bool {
	return c.Logs
}
