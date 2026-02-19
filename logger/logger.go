// Package logger 定义 go-micro 的基础日志级别类型。
package logger

const (
	// Info 表示信息级别日志。
	Info LogLevel = "info"
	// Error 表示错误级别日志。
	Error LogLevel = "error"
	// Success 表示成功级别日志。
	Success LogLevel = "success"
)

// LogLevel 表示日志级别。
type LogLevel string

// AccessLogger 表示访问日志。
type AccessLogger struct {
	Method   uint32 `json:"method"`
	Path     string `json:"path"`
	Request  string `json:"request"`
	Response string `json:"response"`
	Duration uint64 `json:"duration"`
	Status   uint32 `json:"status"`

	ClientIp        string `json:"client_ip"`
	SourceIp        string `json:"source_ip"`
	SourceIpAddress string `json:"source_ip_address"`

	ClientType    uint32 `json:"client_type"`
	ClientName    string `json:"client_name"`
	ClientVersion string `json:"client_version"`

	SystemType    uint32 `json:"system_type"`
	SystemName    string `json:"system_name"`
	SystemVersion string `json:"system_version"`
	AppVersion    string `json:"app_version"`

	InvokeServiceAppId    string `json:"invoke_service_app_id"`
	InvokeServiceEndpoint string `json:"invoke_service_endpoint"`
	TargetServiceAppId    string `json:"target_service_app_id"`
	TargetServiceEndpoint string `json:"target_service_endpoint"`

	TraceId  string `json:"trace_id"`
	UserId   string `json:"user_id"`
	AppId    string `json:"app_id"`
	TenantId string `json:"tenant_id"`
}
