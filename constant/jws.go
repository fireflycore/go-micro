package constant

const (
	// JWSAlgorithmEdDSA 固定当前签名算法，不接受入站自由协商。
	JWSAlgorithmEdDSA = "EdDSA"
	// JWSTypeJWT 是受保护头要求的类型值。
	JWSTypeJWT = "JWT"
	// AuthzSignDefaultKid 是配置装配层的默认公钥标识。
	AuthzSignDefaultKid = "default"
	// AuthzSignDefaultIssuer 是框架默认签发方。
	AuthzSignDefaultIssuer = "firefly-authz"
	// AuthzSignDecisionAllow 是业务入口唯一可接受的授权结论。
	AuthzSignDecisionAllow = "allow"
)

const (
	// AuthzSignClaimIssuer 是签发方声明。
	AuthzSignClaimIssuer = "iss"
	// AuthzSignClaimSubject 是本跳授权主体标识。
	AuthzSignClaimSubject = "sub"
	// AuthzSignClaimSubjectType 区分用户、服务与匿名主体。
	AuthzSignClaimSubjectType = "subject_type"
	// AuthzSignClaimSubjectAppId 是实际参与授权的主体应用。
	AuthzSignClaimSubjectAppId = "subject_app_id"
	// AuthzSignClaimMethod 是授权 API 动作，不是投递方法。
	AuthzSignClaimMethod = "method"
	// AuthzSignClaimPath 是授权 API 路径，不回填实际服务入口。
	AuthzSignClaimPath = "path"
	// AuthzSignClaimServiceAppId 绑定接收证明的本地服务应用。
	AuthzSignClaimServiceAppId = "service_app_id"
	// AuthzSignClaimServiceMethod 绑定实际投递方法。
	AuthzSignClaimServiceMethod = "service_method"
	// AuthzSignClaimServicePath 绑定实际投递入口。
	AuthzSignClaimServicePath = "service_path"
	// AuthzSignClaimUserContext 承载独立的四字段用户视图。
	AuthzSignClaimUserContext = "user_context"
	// AuthzSignClaimInvokeServiceAppId 是上一跳已认证服务，不等于所有场景的授权主体。
	AuthzSignClaimInvokeServiceAppId = "invoke_service_app_id"
	// AuthzSignClaimDecision 是本跳授权结论。
	AuthzSignClaimDecision = "decision"
	// AuthzSignClaimDecisionId 是本跳决策关联标识。
	AuthzSignClaimDecisionId = "decision_id"
	// AuthzSignClaimTraceId 只用于签发侧链路观测。
	AuthzSignClaimTraceId = "trace_id"
	// AuthzSignClaimIssuedAt 是 Unix 秒级签发时间。
	AuthzSignClaimIssuedAt = "iat"
	// AuthzSignClaimNotBefore 是可选的 Unix 秒级生效时间。
	AuthzSignClaimNotBefore = "nbf"
	// AuthzSignClaimExpiresAt 是 Unix 秒级过期时间，声明时长固定为 60 秒。
	AuthzSignClaimExpiresAt = "exp"
)
