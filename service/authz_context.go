package service

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/fireflycore/go-micro/constant"
)

const authzDecisionAllow = "allow"

var (
	// ErrAuthzSignMissing 表示入口请求没有携带 authz 注入的 compact JWS。
	ErrAuthzSignMissing = errors.New("authz sign is missing")
	// ErrAuthzSignMalformed 表示输入不是合法的 compact JWS。
	ErrAuthzSignMalformed = errors.New("authz sign is malformed")
	// ErrAuthzSignPublicKeyMissing 表示无法根据 kid 或默认配置找到验签公钥。
	ErrAuthzSignPublicKeyMissing = errors.New("authz sign public key is missing")
	// ErrAuthzSignUnsupportedAlg 表示 JWS alg 不是当前约定的签名算法。
	ErrAuthzSignUnsupportedAlg = errors.New("authz sign alg is unsupported")
	// ErrAuthzSignInvalidSignature 表示 JWS 签名验证失败。
	ErrAuthzSignInvalidSignature = errors.New("authz sign signature is invalid")
	// ErrAuthzSignInvalidClaims 表示 JWS claim 缺少必要字段或与本地期望不一致。
	ErrAuthzSignInvalidClaims = errors.New("authz sign claims are invalid")
	// ErrAuthzSignExpired 表示 JWS 已过期。
	ErrAuthzSignExpired = errors.New("authz sign is expired")
	// ErrAuthzSignNotYetValid 表示 JWS 尚未到可用时间。
	ErrAuthzSignNotYetValid = errors.New("authz sign is not yet valid")
)

// AuthzSign 表示 x-firefly-authz-sign compact JWS 验签通过后的 payload。
//
// 它不是传输层 metadata；跨进程传输形式始终是 constant.AuthzSign 对应的 compact JWS。
type AuthzSign struct {
	// KeyId 是 JWS header 中的 kid，用于密钥轮换排查。
	KeyId string `json:"-"`
	// Issuer 表示签发方，通常由业务服务配置指定期望值。
	Issuer string `json:"iss"`
	// SubjectId 表示进入 Casbin 的主体 ID，当前通常等于 invoke_app_id 或 anonymous。
	SubjectId string `json:"sub"`
	// SubjectType 表示主体类型，取值见 constant.SubjectTypeAnonymous/User/Service。
	SubjectType string `json:"subject_type"`
	// UserId 表示用户主体 ID，服务或匿名主体为空，来自 user_context.user_id。
	UserId string `json:"user_id,omitempty"`
	// AppId 表示用户身份中的应用 ID，来自 user_context.app_id。
	AppId string `json:"app_id,omitempty"`
	// Session 表示 authz 从 token 中解析出的会话标识。
	Session string `json:"session,omitempty"`
	// TenantId 表示用户所属租户 ID，来自 user_context.tenant_id。
	TenantId string `json:"tenant_id,omitempty"`
	// OrgIds 表示用户关联的组织 ID 列表。
	OrgIds []string `json:"org_ids,omitempty"`
	// PostIds 表示用户关联的岗位 ID 列表。
	PostIds []string `json:"post_ids,omitempty"`
	// RoleIds 表示用户关联的角色 ID 列表。
	RoleIds []string `json:"role_ids,omitempty"`
	// InvokeAppId 表示本跳权限判定中的调用方应用 ID。
	InvokeAppId string `json:"invoke_app_id"`
	// InvokeInstanceId 表示本跳调用服务实例 ID，可为空。
	InvokeInstanceId string `json:"invoke_instance_id,omitempty"`
	// TargetAppId 表示 authz 对 route.app_id 的判定语义。
	TargetAppId string `json:"target_app_id"`
	// TargetInstanceId 表示 route.instance_id 映射出的目标服务实例 ID，可为空。
	TargetInstanceId string `json:"target_instance_id,omitempty"`
	// ApiMethod 表示本次授权动作，取值见 constant.RequestMethod*String。
	ApiMethod string `json:"api_method"`
	// ApiPath 表示本次授权资源路径，HTTP 为入口 path，gRPC 为 FullMethod。
	ApiPath string `json:"api_path"`
	// Decision 表示 authz 判定结果，当前只接受 allow。
	Decision string `json:"decision"`
	// DecisionId 表示 authz 对本次判定生成的唯一 ID。
	DecisionId string `json:"decision_id"`
	// TraceId 表示 authz 从 traceparent 中提取的 OTel trace_id。
	TraceId string `json:"trace_id,omitempty"`
	// UserContext 保存 authz 从用户 authority 还原出的用户身份上下文。
	UserContext *AuthzUserContext `json:"user_context,omitempty"`
	// InvokeServiceContext 保存 authz 从服务 authority 还原出的当前跳调用服务身份。
	InvokeServiceContext *AuthzInvokeServiceContext `json:"invoke_service_context,omitempty"`
	// TargetServiceContext 保存 authz 从 route 事实映射出的被访问服务身份。
	TargetServiceContext *AuthzTargetServiceContext `json:"target_service_context,omitempty"`
	// IssuedAt 表示签发时间，Unix 秒。
	IssuedAt int64 `json:"iat"`
	// NotBefore 表示最早可用时间，Unix 秒；当前 authz 可不写。
	NotBefore int64 `json:"nbf,omitempty"`
	// ExpiresAt 表示过期时间，Unix 秒。
	ExpiresAt int64 `json:"exp"`
}

// AuthzUserContext 表示 JWS payload 中 user_context 的结构化用户身份。
type AuthzUserContext struct {
	// UserId 是用户主体 ID。
	UserId string `json:"user_id,omitempty"`
	// AppId 是用户身份中的应用 ID，不等同于本跳服务调用方 app_id。
	AppId string `json:"app_id,omitempty"`
	// TenantId 是用户所属租户 ID。
	TenantId string `json:"tenant_id,omitempty"`
	// Session 是用户 token 关联的会话标识。
	Session string `json:"session,omitempty"`
	// OrgIds 是用户关联组织 ID 列表。
	OrgIds []string `json:"org_ids,omitempty"`
	// PostIds 是用户关联岗位 ID 列表。
	PostIds []string `json:"post_ids,omitempty"`
	// RoleIds 是用户关联角色 ID 列表。
	RoleIds []string `json:"role_ids,omitempty"`
}

// AuthzInvokeServiceContext 表示 JWS payload 中 invoke_service_context 的结构化服务身份。
type AuthzInvokeServiceContext struct {
	// AppId 是当前这一跳调用方服务的应用 ID。
	AppId string `json:"app_id,omitempty"`
	// InstanceId 是当前这一跳调用方服务实例 ID，可为空。
	InstanceId string `json:"instance_id,omitempty"`
}

// AuthzTargetServiceContext 表示 JWS payload 中 target_service_context 的结构化目标服务身份。
type AuthzTargetServiceContext struct {
	// AppId 是 route 所属服务 app_id，在 authz 判定中才解释为 target_app_id。
	AppId string `json:"app_id,omitempty"`
	// InstanceId 是 route 所属服务实例 ID，可为空。
	InstanceId string `json:"instance_id,omitempty"`
}

// AuthzSignVerificationOptions 定义服务侧本地验签 x-firefly-authz-sign 的规则。
type AuthzSignVerificationOptions struct {
	// PublicKey 是单公钥模式下的 Ed25519 公钥。
	PublicKey ed25519.PublicKey
	// PublicKeys 是按 kid 索引的 Ed25519 公钥集合，用于密钥轮换。
	PublicKeys map[string]ed25519.PublicKey
	// Issuer 非空时要求 iss 必须与该值一致。
	Issuer string
	// ExpectedTargetAppId 非空时要求 target_app_id 必须等于当前服务 app_id。
	ExpectedTargetAppId string
	// ExpectedApiMethod 非空时要求 api_method 必须匹配当前入口授权动作。
	ExpectedApiMethod string
	// ExpectedApiPath 非空时要求 api_path 必须匹配当前入口资源路径。
	ExpectedApiPath string
	// ClockSkew 表示允许的本机时钟偏差。
	ClockSkew time.Duration
	// Now 允许测试注入固定时间；生产环境留空使用 time.Now。
	Now func() time.Time
}

type authzSignJWSHeader struct {
	// Alg 表示 JWS 签名算法，当前只接受 constant.JWSAlgorithmEdDSA。
	Alg string `json:"alg"`
	// Kid 表示签名密钥 ID，用于从 PublicKeys 中选择公钥。
	Kid string `json:"kid"`
	// Typ 表示 token 类型，通常为 constant.JWSTypeJWT；当前不参与判定。
	Typ string `json:"typ"`
}

// VerifyAuthzSign 校验 authz compact JWS 并返回可信 payload。
func VerifyAuthzSign(raw string, options AuthzSignVerificationOptions) (*AuthzSign, error) {
	// 先裁剪外部传入值，避免 header 前后空白影响 compact JWS 解析。
	raw = strings.TrimSpace(raw)
	// 没有 JWS 时直接返回明确错误，由入口决定是否允许跳过。
	if raw == "" {
		return nil, ErrAuthzSignMissing
	}

	// compact JWS 固定由 header.payload.signature 三段组成。
	parts := strings.Split(raw, ".")
	// 任意一段缺失都说明不是合法 JWS，直接拒绝。
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, ErrAuthzSignMalformed
	}

	// 先解析 header，拿到 alg 和 kid 后才能选择验签策略。
	header, err := parseAuthzSignHeader(parts[0])
	if err != nil {
		return nil, err
	}
	// 当前 authz 约定使用 Ed25519/EdDSA，拒绝 alg 降级或替换。
	if header.Alg != constant.JWSAlgorithmEdDSA {
		return nil, ErrAuthzSignUnsupportedAlg
	}

	// 根据 kid 选择公钥；未配置多公钥时退化为单公钥模式。
	publicKey, ok := resolveAuthzSignPublicKey(header.Kid, options)
	if !ok {
		return nil, ErrAuthzSignPublicKeyMissing
	}

	// 签名段必须是 base64url，无 padding。
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrAuthzSignMalformed
	}
	// Ed25519 的签名输入必须是原始 header.payload 字符串，不能用解码后的 JSON 重组。
	if !ed25519.Verify(publicKey, []byte(parts[0]+"."+parts[1]), signature) {
		return nil, ErrAuthzSignInvalidSignature
	}

	// 签名通过后再解码 payload，避免在未可信数据上做多余业务处理。
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrAuthzSignMalformed
	}

	// 先把 kid 写入上下文，便于日志或排障时知道使用的是哪把公钥。
	claims := &AuthzSign{KeyId: header.Kid}
	// 将 JSON claim 反序列化为稳定结构，避免业务侧直接操作 map。
	if err := json.Unmarshal(payload, claims); err != nil {
		return nil, ErrAuthzSignMalformed
	}
	// 先补齐结构化 claim 派生出的平铺字段，再做必要字段和本地期望校验。
	normalizeVerifiedAuthzSign(claims)
	// 最后校验 issuer、时间窗口和当前入口期望的资源事实。
	if err := validateAuthzSignClaims(claims, options); err != nil {
		return nil, err
	}
	// 返回已经验签和校验过的可信 payload。
	return claims, nil
}

func parseAuthzSignHeader(segment string) (*authzSignJWSHeader, error) {
	// JWS header 使用 base64url 编码。
	data, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return nil, ErrAuthzSignMalformed
	}
	// 解析出 alg/kid/typ，后续用 alg 和 kid 做验签约束。
	header := &authzSignJWSHeader{}
	if err := json.Unmarshal(data, header); err != nil {
		return nil, ErrAuthzSignMalformed
	}
	// 返回原始 header 结构，不在这里做业务校验。
	return header, nil
}

func resolveAuthzSignPublicKey(kid string, options AuthzSignVerificationOptions) (ed25519.PublicKey, bool) {
	// 多公钥模式优先，便于 authz 密钥轮换期间同时接受新旧 kid。
	if len(options.PublicKeys) > 0 {
		// 通过 JWS header 中的 kid 找到对应公钥。
		key, ok := options.PublicKeys[kid]
		// Ed25519 公钥必须是固定长度，否则视为配置错误。
		return key, ok && len(key) == ed25519.PublicKeySize
	}
	// 单公钥模式用于简单部署或测试环境。
	if len(options.PublicKey) == ed25519.PublicKeySize {
		return options.PublicKey, true
	}
	// 没有可用公钥时返回 false，由上层转换成明确错误。
	return nil, false
}

func validateAuthzSignClaims(claims *AuthzSign, options AuthzSignVerificationOptions) error {
	// claim 为空说明 payload 没有成功构造成可信上下文。
	if claims == nil {
		return ErrAuthzSignInvalidClaims
	}
	// 配置了 issuer 时必须严格匹配，防止其他签发方 token 混入。
	if options.Issuer != "" && claims.Issuer != options.Issuer {
		return ErrAuthzSignInvalidClaims
	}
	// 主体、目标应用、授权动作和资源路径是服务侧验签后的最小可信字段。
	if claims.SubjectId == "" || claims.SubjectType == "" || claims.TargetAppId == "" || claims.ApiMethod == "" || claims.ApiPath == "" {
		return ErrAuthzSignInvalidClaims
	}
	// 非匿名请求必须有调用方 app_id；公共接口允许 invoke_app_id 为空。
	if claims.SubjectType != constant.SubjectTypeAnonymous && claims.InvokeAppId == "" {
		return ErrAuthzSignInvalidClaims
	}
	// 当前只有 allow 结果会被注入业务服务，其他 decision 一律拒绝。
	if claims.Decision != "" && claims.Decision != authzDecisionAllow {
		return ErrAuthzSignInvalidClaims
	}

	// 默认使用系统时间，测试可通过 options.Now 注入固定时间。
	now := time.Now
	if options.Now != nil {
		now = options.Now
	}
	// 统一转 UTC，避免本地时区影响 Unix 秒比较。
	current := now().UTC()
	// ClockSkew 用于容忍极小范围的机器时钟偏差。
	skew := options.ClockSkew
	// exp 是必需字段；当前时间超过 exp 后 token 失效。
	if claims.ExpiresAt <= 0 || !current.Add(-skew).Before(time.Unix(claims.ExpiresAt, 0)) {
		return ErrAuthzSignExpired
	}
	// nbf 可选；写入时表示当前时间必须晚于 nbf。
	if claims.NotBefore > 0 && current.Add(skew).Before(time.Unix(claims.NotBefore, 0)) {
		return ErrAuthzSignNotYetValid
	}
	// 目标 app_id 非空时必须匹配当前服务，避免把 A 服务的授权结果拿到 B 服务复用。
	if options.ExpectedTargetAppId != "" && claims.TargetAppId != options.ExpectedTargetAppId {
		return ErrAuthzSignInvalidClaims
	}
	// 授权动作非空时必须匹配当前入口；gRPC 场景通常是 GRPC。
	if options.ExpectedApiMethod != "" && !strings.EqualFold(claims.ApiMethod, options.ExpectedApiMethod) {
		return ErrAuthzSignInvalidClaims
	}
	// 资源路径非空时必须匹配当前 FullMethod 或 HTTP path。
	if options.ExpectedApiPath != "" && claims.ApiPath != options.ExpectedApiPath {
		return ErrAuthzSignInvalidClaims
	}
	// 所有必要 claim 和本地期望都通过后，认为上下文可信。
	return nil
}

func normalizeVerifiedAuthzSign(claims *AuthzSign) {
	// 空 payload 无需处理，调用方会在 validateAuthzSignClaims 中拒绝。
	if claims == nil {
		return
	}
	// user_context 是目标结构；平铺 user_id/app_id/tenant_id 是业务读取便利字段。
	if claims.UserContext != nil {
		if claims.UserId == "" {
			claims.UserId = claims.UserContext.UserId
		}
		if claims.AppId == "" {
			claims.AppId = claims.UserContext.AppId
		}
		if claims.TenantId == "" {
			claims.TenantId = claims.UserContext.TenantId
		}
		if claims.Session == "" {
			claims.Session = claims.UserContext.Session
		}
		if len(claims.OrgIds) == 0 {
			claims.OrgIds = cloneStrings(claims.UserContext.OrgIds)
		}
		if len(claims.PostIds) == 0 {
			claims.PostIds = cloneStrings(claims.UserContext.PostIds)
		}
		if len(claims.RoleIds) == 0 {
			claims.RoleIds = cloneStrings(claims.UserContext.RoleIds)
		}
	}
	// invoke_service_context 保持服务 authority 原始语义，平铺 invoke_* 是验签快捷字段。
	if claims.InvokeServiceContext != nil {
		if claims.InvokeAppId == "" {
			claims.InvokeAppId = claims.InvokeServiceContext.AppId
		}
		if claims.InvokeInstanceId == "" {
			claims.InvokeInstanceId = claims.InvokeServiceContext.InstanceId
		}
	}
	// 用户首跳没有 service authority 时，authz 可以用 UserContext.app_id 作为 invoke_app_id。
	if claims.InvokeAppId == "" && claims.UserContext != nil {
		claims.InvokeAppId = claims.UserContext.AppId
	}
	// target_service_context 来自 route.app_id/instance_id，平铺 target_* 是验签快捷字段。
	if claims.TargetServiceContext != nil {
		if claims.TargetAppId == "" {
			claims.TargetAppId = claims.TargetServiceContext.AppId
		}
		if claims.TargetInstanceId == "" {
			claims.TargetInstanceId = claims.TargetServiceContext.InstanceId
		}
	}
}
