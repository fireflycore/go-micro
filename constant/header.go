// Package constant 定义微服务通用 header/metadata key。
package constant

const (
	// XRealIp 真实ip，一般由nginx设置的
	XRealIp = "x-real-ip"
	// Authorization 用户token
	Authorization = "authorization"

	// HeaderPrefix Firefly系统自定义头部（统一前缀）
	HeaderPrefix = "x-firefly-"

	// AccessMethod 访问方式（http2grpc[http-gateway->grpc-gateway], grpc2grpc[grpc-gateway->grpc-service]）
	AccessMethod          = HeaderPrefix + "access-method"
	AccessMethodHTTP2GRPC = "http2grpc"
	AccessMethodGRPC2GRPC = "grpc2grpc"

	// TraceId 链路id
	TraceId = HeaderPrefix + "trace-id"

	// AppLanguage 应用语言
	AppLanguage = HeaderPrefix + "app-language"
	// AppVersion 应用版本
	AppVersion = HeaderPrefix + "app-version"

	// Session 用户session
	Session = HeaderPrefix + "session"
	// UserId 用户id
	UserId = HeaderPrefix + "user-id"
	// AppId 用户当前的应用Id
	AppId = HeaderPrefix + "app-id"
	// TenantId 用户当前应用的归属租户id
	TenantId = HeaderPrefix + "tenant-id"
	// OrgIds 用户当前应用下的组织Ids
	OrgIds = HeaderPrefix + "org-ids"
	// RoleIds 用户当前应用下的角色Ids
	RoleIds = HeaderPrefix + "role-ids"

	// SystemType 客户端系统类型
	SystemType = HeaderPrefix + "system-type"
	// SystemName 客户端系统名称
	SystemName = HeaderPrefix + "system-name"
	// SystemVersion 客户端系统版本
	SystemVersion = HeaderPrefix + "system-version"
	// ClientType 客户端类型
	ClientType = HeaderPrefix + "client-type"
	// ClientName 客户端名称
	ClientName = HeaderPrefix + "client-name"
	// ClientVersion 客户端版本
	ClientVersion = HeaderPrefix + "client-version"

	// GatewayAuth 网关认证
	GatewayAuth = HeaderPrefix + "gateway-auth"

	// InvokeServiceAuth 服务调用相关（Invoke-服务调用方信息，Target-被调用方服务信息）
	InvokeServiceAuth     = HeaderPrefix + "invoke-service-auth"
	InvokeServiceAppId    = HeaderPrefix + "invoke-service-app-id"
	InvokeServiceEndpoint = HeaderPrefix + "invoke-service-endpoint"
	TargetServiceAppId    = HeaderPrefix + "target-service-app-id"
	TargetServiceEndpoint = HeaderPrefix + "target-service-endpoint"

	ClientIp = HeaderPrefix + "client-ip"
	SourceIp = HeaderPrefix + "source-ip"
)
