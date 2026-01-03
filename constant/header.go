// Package constant 定义微服务通用 header/metadata key。
package constant

const (
	XRealIp           = "x-real-ip"
	Authorization     = "authorization"
	AuthorizationType = "authorization-type"

	AppLanguage = "app-language"
	AppVersion  = "app-version"

	TraceId  = "trace-id"
	Session  = "session"
	UserId   = "user-id"
	AppId    = "app-id"
	TenantId = "tenant-id"
	ClientIp = "client-ip"

	RoleIds = "role-ids"
	OrgIds  = "org-ids"

	SystemName       = "system-name"
	ClientName       = "client-name"
	SystemType       = "system-type"
	ClientType       = "client-type"
	DeviceFormFactor = "device-form-factor"
	SystemVersion    = "system-version"
	ClientVersion    = "client-version"

	GrpcGatewayAuth = "grpc-gateway-auth"
	HttpGatewayAuth = "http-gateway-auth"
)
