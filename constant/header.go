// Package constant 定义微服务通用 header/metadata key。
package constant

const (
	XRealIp           = "x-real-ip"
	Authorization     = "authorization"
	AuthorizationType = "authorization-type"

	// HeaderPrefix Firefly系统自定义头部（统一前缀）
	HeaderPrefix = "x-firefly-"

	// 应用相关

	AppLanguage = HeaderPrefix + "app-language"
	AppVersion  = HeaderPrefix + "app-version"

	// 用户上下文

	TraceId  = HeaderPrefix + "trace-id"
	Session  = HeaderPrefix + "session"
	UserId   = HeaderPrefix + "user-id"
	AppId    = HeaderPrefix + "app-id"
	TenantId = HeaderPrefix + "tenant-id"
	ClientIp = HeaderPrefix + "client-ip"
	RoleIds  = HeaderPrefix + "role-ids"
	OrgIds   = HeaderPrefix + "org-ids"

	// 设备/客户端信息

	SystemName       = HeaderPrefix + "system-name"
	ClientName       = HeaderPrefix + "client-name"
	SystemType       = HeaderPrefix + "system-type"
	ClientType       = HeaderPrefix + "client-type"
	DeviceFormFactor = HeaderPrefix + "device-form-factor"
	SystemVersion    = HeaderPrefix + "system-version"
	ClientVersion    = HeaderPrefix + "client-version"

	// GatewayAuth 网关认证
	GatewayAuth = HeaderPrefix + "gateway-auth"

	// 服务调用相关（Invoke-服务调用方信息，Target-被调用方服务信息）

	InvokeServiceAuth     = HeaderPrefix + "invoke-service-auth"
	InvokeServiceAppId    = HeaderPrefix + "invoke-service-app-id"
	InvokeServiceEndpoint = HeaderPrefix + "invoke-service-endpoint"
	TargetServiceAppId    = HeaderPrefix + "target-service-app-id"
	TargetServiceEndpoint = HeaderPrefix + "target-service-endpoint"
)
