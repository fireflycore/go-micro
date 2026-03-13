package telemetry

type Conf struct {
	ServiceName    string
	ServiceVersion string

	OTLPEndpoint string
	Insecure     bool

	Traces  bool
	Logs    bool
	Metrics bool
}
