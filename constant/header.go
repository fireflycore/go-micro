// Package constant 定义微服务通用 header/metadata key。
package constant

const (
	XRealIp           = "x-real-ip"
	Authorization     = "authorization"
	AuthorizationType = "authorization-type"

	AppLanguage = "ff-app-language"
	AppVersion  = "ff-app-version"

	TraceId  = "ff-trace-id"
	Session  = "ff-session"
	UserId   = "ff-user-id"
	AppId    = "ff-app-id"
	TenantId = "ff-tenant-id"
	ClientIp = "ff-client-ip"

	RoleIds = "ff-role-ids"
	OrgIds  = "ff-org-ids"

	SystemName       = "ff-system-name"
	ClientName       = "ff-client-name"
	SystemType       = "ff-system-type"
	ClientType       = "ff-client-type"
	DeviceFormFactor = "ff-device-form-factor"
	SystemVersion    = "ff-system-version"
	ClientVersion    = "ff-client-version"

	GrpcGatewayAuth = "ff-grpc-gateway-auth"
	HttpGatewayAuth = "ff-http-gateway-auth"
)
