package logger

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
	ParentId string `json:"parent_id"`
	SpanId   string `json:"span_id"`

	UserId   string `json:"user_id"`
	AppId    string `json:"app_id"`
	TenantId string `json:"tenant_id"`
}

// ServerLogger 表示服务端日志。
type ServerLogger struct {
	Path    string `json:"path"`
	Level   uint32 `json:"level"`
	Content string `json:"content"`

	TraceId  string `json:"trace_id"`
	ParentId string `json:"parent_id"`

	UserId   string `json:"user_id"`
	AppId    string `json:"app_id"`
	TenantId string `json:"tenant_id"`
}

// OperationLogger 表示操作日志。
type OperationLogger struct {
	Database  string `json:"database"`
	Statement string `json:"statement"`
	Result    string `json:"result"`
	Path      string `json:"path"`

	Duration uint64 `json:"duration"`

	Level uint32 `json:"level"`
	Type  uint32 `json:"type"`

	TraceId  string `json:"trace_id"`
	ParentId string `json:"parent_id"`

	TargetAppId string `json:"target_app_id"`
	InvokeAppId string `json:"invoke_app_id"`

	UserId   string `json:"user_id"`
	AppId    string `json:"app_id"`
	TenantId string `json:"tenant_id"`
}
