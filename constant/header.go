// Package constant 定义 Firefly 微服务通信中稳定使用的 HTTP Header / gRPC Metadata key。
package constant

const (
	// XRealIp 是代理输入；只有显式验证代理可信后才能提取，不通用透传。
	XRealIp = "x-real-ip"
	// XForwardedFor 是代理链输入，必须结合连接对端和可信代理策略解释。
	XForwardedFor = "x-forwarded-for"
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
	// AppLanguage 表示客户端应用语言偏好，可作为访问日志和业务展示上下文使用。
	AppLanguage = HeaderPrefix + "app-language"
	// AppVersion 表示客户端应用版本。
	AppVersion = HeaderPrefix + "app-version"
)

const (
	// SubjectType 表示 authz 注入的普通主体类型，取值见 SubjectTypeAnonymous/User/Service。
	SubjectType = HeaderPrefix + "subject-type"
	// DecisionId 表示 authz 对本次 allow 判定生成的普通决策 ID，用于日志关联。
	DecisionId = HeaderPrefix + "decision-id"
	// AuthzSign 表示 authz 写入的短有效期 compact JWS，是服务侧本地验签输入。
	AuthzSign = HeaderPrefix + "authz-sign"
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
	// UserAuthority 表示跨进程传输的用户 authority 原文，只由 authz 负责校验和解析。
	UserAuthority = HeaderPrefix + "user-authority"
	// ServiceAuthority 表示跨进程传输的服务 authority 原文，每一跳由当前调用服务覆盖写入。
	ServiceAuthority = HeaderPrefix + "service-authority"
)

const (
	// AcceptLanguage 仅作为入站语言备用输入，出站统一使用 AppLanguage。
	AcceptLanguage = "accept-language"
	// InvokeMethod 是上一跳上报方法，仅供观测，不是授权动作。
	InvokeMethod = HeaderPrefix + "invoke-method"
	// InvokePath 是上一跳上报入口，不是实际入口或本次目标。
	InvokePath = HeaderPrefix + "invoke-path"
)

// 以下明文头仅供观测；业务 UserContext 必须从验签后的 payload 构建。
const (
	// UserId 是明文用户标识观测头；服务主体携带用户时也可存在。
	UserId = HeaderPrefix + "user-id"
	// Session 是明文会话观测头，不能据此建立会话或默认写入日志。
	Session = HeaderPrefix + "session"
	// AppId 表示用户身份中的应用 ID，不表达服务调用方。
	AppId = HeaderPrefix + "app-id"
	// TenantId 表示用户所属租户 ID。
	TenantId = HeaderPrefix + "tenant-id"
)
