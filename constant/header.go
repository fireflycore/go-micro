// Package constant 定义微服务通用 header/metadata key。
package constant

const (
	// 标准头
	XRealIp           = "x-real-ip"
	Authorization     = "authorization"
	AuthorizationType = "authorization-type"

	// HeaderPrefix Firefly系统自定义头部（统一前缀）
	HeaderPrefix = "x-firefly-"

	// AccessMethod 访问方式（http2grpc[http-gateway->grpc-gateway], grpc2grpc[grpc-gateway->grpc-service]）
	AccessMethod          = HeaderPrefix + "access-method"
	AccessMethodHTTP2GRPC = "http2grpc"
	AccessMethodGRPC2GRPC = "grpc2grpc"

	// TraceId 链路id
	TraceId = HeaderPrefix + "trace-id"

	// 应用相关
	AppLanguage = HeaderPrefix + "app-language"
	AppVersion  = HeaderPrefix + "app-version"

	// 用户上下文
	Session  = HeaderPrefix + "session"
	UserId   = HeaderPrefix + "user-id"
	AppId    = HeaderPrefix + "app-id"
	TenantId = HeaderPrefix + "tenant-id"

	OrgIds  = HeaderPrefix + "org-ids"
	RoleIds = HeaderPrefix + "role-ids"

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

	ClientIp = HeaderPrefix + "client-ip"
	SourceIp = HeaderPrefix + "source-ip"
)
