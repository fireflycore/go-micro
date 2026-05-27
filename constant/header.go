// Package constant 定义 Firefly 微服务通信中稳定使用的 HTTP Header / gRPC Metadata key。
package constant

const (
	// XRealIp 表示入口代理透传的真实客户端 IP，通常由 Nginx / Ingress 写入。
	XRealIp = "x-real-ip"
	// XForwardedFor 表示标准代理链路 IP 列表，入口服务只读取第一个有效地址。
	XForwardedFor = "x-forwarded-for"
	// Authorization 表示标准认证头，当前由 authz 在数据面统一消费。
	Authorization = "authorization"
	// TraceParent 是 W3C Trace Context 中承载 trace_id/span_id/parent 关系的标准头。
	TraceParent = "traceparent"
	// TraceState 是 W3C Trace Context 中承载厂商扩展 trace 状态的标准头。
	TraceState = "tracestate"
	// Baggage 是 W3C Baggage 标准头，用于跨进程传播低基数业务上下文。
	Baggage = "baggage"

	// HeaderPrefix 是 Firefly 自定义 header 的统一前缀。
	HeaderPrefix = "x-firefly-"
)

const (
	// AppLanguage 表示客户端应用语言偏好。
	AppLanguage = HeaderPrefix + "app-language"
	// AppVersion 表示客户端应用版本。
	AppVersion = HeaderPrefix + "app-version"

	// Session 表示用户或服务会话标识，仅由可信入口写入。
	Session = HeaderPrefix + "session"
	// UserId 表示当前用户主体 ID；服务或匿名主体为空。
	UserId = HeaderPrefix + "user-id"
	// AppId 表示调用方应用 ID；authz allow 后与 InvokeAppId 保持一致，保留给业务侧读取。
	AppId = HeaderPrefix + "app-id"
	// TenantId 表示当前主体所属租户 ID。
	TenantId = HeaderPrefix + "tenant-id"
	// OrgIds 表示当前主体关联的组织 ID 列表。
	OrgIds = HeaderPrefix + "org-ids"
	// RoleIds 表示当前主体关联的角色 ID 列表。
	RoleIds = HeaderPrefix + "role-ids"
)

const (
	// SubjectType 表示本次请求的主体类型，取值见 SubjectTypeAnonymous/User/Service。
	SubjectType = HeaderPrefix + "subject-type"
	// SubjectTypeAnonymous 表示无 token 但命中公共策略的匿名主体。
	SubjectTypeAnonymous = "anonymous"
	// SubjectTypeUser 表示通过用户 token 还原出的用户主体。
	SubjectTypeUser = "user"
	// SubjectTypeService 表示通过服务 session token 还原出的服务主体。
	SubjectTypeService = "service"
	// InvokeAppId 表示发起调用的应用 ID，由 authz 根据 token/session 可信解析。
	InvokeAppId = HeaderPrefix + "invoke-app-id"
	// TargetAppId 表示被访问资源所属应用 ID，由 route context 进入 authz 后签名注入。
	TargetAppId = HeaderPrefix + "target-app-id"
	// ResourceType 表示本次授权动作，HTTP 为 GET/POST/PUT/DELETE，gRPC 为 GRPC。
	ResourceType = HeaderPrefix + "resource-type"
	// ResourcePath 表示本次授权资源路径，HTTP 为入口 path，gRPC 为 /package.Service/Method。
	ResourcePath = HeaderPrefix + "resource-path"
	// DecisionId 表示 authz 对本次 allow 判定生成的唯一决策 ID，用于审计串联。
	DecisionId = HeaderPrefix + "decision-id"
	// AuthzContext 表示 authz 写入的短有效期签名上下文 JWS，是服务侧信任根。
	AuthzContext = HeaderPrefix + "authz-context"
)

const (
	// SystemType 表示客户端操作系统类型枚举值。
	SystemType = HeaderPrefix + "system-type"
	// SystemName 表示客户端操作系统名称。
	SystemName = HeaderPrefix + "system-name"
	// SystemVersion 表示客户端操作系统版本。
	SystemVersion = HeaderPrefix + "system-version"
	// ClientType 表示客户端类型枚举值。
	ClientType = HeaderPrefix + "client-type"
	// ClientName 表示客户端名称。
	ClientName = HeaderPrefix + "client-name"
	// ClientVersion 表示客户端版本。
	ClientVersion = HeaderPrefix + "client-version"
)

const (
	// ServiceAppId 表示当前发起下游 gRPC 调用的服务应用标识，由 go-micro/invocation 注入。
	ServiceAppId = HeaderPrefix + "service-app-id"
	// ServiceInstanceId 表示当前发起下游 gRPC 调用的服务实例标识，由 go-micro/invocation 注入。
	ServiceInstanceId = HeaderPrefix + "service-instance-id"
)
