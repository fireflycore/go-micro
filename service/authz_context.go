package service

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	// ErrAuthzContextMissing 表示入口请求没有携带 authz 注入的签名上下文。
	ErrAuthzContextMissing = errors.New("authz context is missing")
	// ErrAuthzContextMalformed 表示签名上下文不是合法的 compact JWS。
	ErrAuthzContextMalformed = errors.New("authz context is malformed")
	// ErrAuthzContextPublicKeyMissing 表示无法根据 kid 或默认配置找到验签公钥。
	ErrAuthzContextPublicKeyMissing = errors.New("authz context public key is missing")
	// ErrAuthzContextUnsupportedAlg 表示 JWS alg 不是当前约定的 EdDSA。
	ErrAuthzContextUnsupportedAlg = errors.New("authz context alg is unsupported")
	// ErrAuthzContextInvalidSignature 表示 JWS 签名验证失败。
	ErrAuthzContextInvalidSignature = errors.New("authz context signature is invalid")
	// ErrAuthzContextInvalidClaims 表示 JWS claim 缺少必要字段或与本地期望不一致。
	ErrAuthzContextInvalidClaims = errors.New("authz context claims are invalid")
	// ErrAuthzContextExpired 表示 JWS 已过期。
	ErrAuthzContextExpired = errors.New("authz context is expired")
	// ErrAuthzContextNotYetValid 表示 JWS 尚未到可用时间。
	ErrAuthzContextNotYetValid = errors.New("authz context is not yet valid")
)

// AuthzContext 表示 authz allow 后写入 x-firefly-authz-context 的可信载荷。
type AuthzContext struct {
	// KeyId 是 JWS header 中的 kid，用于审计和密钥轮换排查。
	KeyId string `json:"-"`
	// Issuer 表示签发方，当前约定为 firefly-authz。
	Issuer string `json:"iss"`
	// SubjectId 表示进入 Casbin 的主体 ID，用户为 user_id，服务为 app_id。
	SubjectId string `json:"sub"`
	// SubjectType 表示主体类型，当前取值为 anonymous/user/service。
	SubjectType string `json:"subject_type"`
	// UserId 表示用户主体 ID，服务或匿名主体为空。
	UserId string `json:"user_id,omitempty"`
	// TenantId 表示主体所属租户 ID。
	TenantId string `json:"tenant_id,omitempty"`
	// InvokeAppId 表示发起调用的应用 ID。
	InvokeAppId string `json:"invoke_app_id"`
	// TargetAppId 表示被访问资源所属应用 ID。
	TargetAppId string `json:"target_app_id"`
	// ResourceType 表示本次授权动作，HTTP 为方法名，gRPC 为 GRPC。
	ResourceType string `json:"resource_type"`
	// ResourcePath 表示本次授权资源路径。
	ResourcePath string `json:"path"`
	// RouteId 表示网关侧 route 标识，仅用于审计和排障。
	RouteId string `json:"route_id,omitempty"`
	// Decision 表示 authz 判定结果，当前允许链路固定为 allow。
	Decision string `json:"decision"`
	// DecisionId 表示 authz 对本次判定生成的唯一 ID。
	DecisionId string `json:"decision_id"`
	// TraceId 表示 authz 从 traceparent 中提取的 OTel trace_id。
	TraceId string `json:"trace_id,omitempty"`
	// IssuedAt 表示签发时间，Unix 秒。
	IssuedAt int64 `json:"iat"`
	// NotBefore 表示最早可用时间，Unix 秒；当前 authz 可不写。
	NotBefore int64 `json:"nbf,omitempty"`
	// ExpiresAt 表示过期时间，Unix 秒。
	ExpiresAt int64 `json:"exp"`
}

// AuthzContextVerificationOptions 定义服务侧本地验签 x-firefly-authz-context 的规则。
type AuthzContextVerificationOptions struct {
	// PublicKey 是单公钥模式下的 Ed25519 公钥。
	PublicKey ed25519.PublicKey
	// PublicKeys 是按 kid 索引的 Ed25519 公钥集合，用于密钥轮换。
	PublicKeys map[string]ed25519.PublicKey
	// Issuer 非空时要求 iss 必须与该值一致。
	Issuer string
	// ExpectedTargetAppId 非空时要求 target_app_id 必须等于当前服务 app_id。
	ExpectedTargetAppId string
	// ExpectedResourceType 非空时要求 resource_type 必须匹配当前入口资源动作。
	ExpectedResourceType string
	// ExpectedResourcePath 非空时要求 path 必须匹配当前入口资源路径。
	ExpectedResourcePath string
	// ClockSkew 表示允许的本机时钟偏差。
	ClockSkew time.Duration
	// Now 允许测试注入固定时间；生产环境留空使用 time.Now。
	Now func() time.Time
}

type authzContextJWSHeader struct {
	// Alg 表示 JWS 签名算法，当前只接受 EdDSA。
	Alg string `json:"alg"`
	// Kid 表示签名密钥 ID，用于从 PublicKeys 中选择公钥。
	Kid string `json:"kid"`
	// Typ 表示 token 类型，通常为 JWT；当前不参与判定。
	Typ string `json:"typ"`
}

// VerifyAuthzContext 校验 authz 签名上下文并返回可信 claim。
func VerifyAuthzContext(raw string, options AuthzContextVerificationOptions) (*AuthzContext, error) {
	// 先裁剪外部传入值，避免 header 前后空白影响 compact JWS 解析。
	raw = strings.TrimSpace(raw)
	// 没有签名上下文时直接返回明确错误，由入口决定是否允许跳过。
	if raw == "" {
		return nil, ErrAuthzContextMissing
	}

	// compact JWS 固定由 header.payload.signature 三段组成。
	parts := strings.Split(raw, ".")
	// 任意一段缺失都说明不是合法 JWS，直接拒绝。
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, ErrAuthzContextMalformed
	}

	// 先解析 header，拿到 alg 和 kid 后才能选择验签策略。
	header, err := parseAuthzContextHeader(parts[0])
	if err != nil {
		return nil, err
	}
	// 当前 authz 约定使用 Ed25519/EdDSA，拒绝 alg 降级或替换。
	if header.Alg != "EdDSA" {
		return nil, ErrAuthzContextUnsupportedAlg
	}

	// 根据 kid 选择公钥；未配置多公钥时退化为单公钥模式。
	publicKey, ok := resolveAuthzContextPublicKey(header.Kid, options)
	if !ok {
		return nil, ErrAuthzContextPublicKeyMissing
	}

	// 签名段必须是 base64url，无 padding。
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrAuthzContextMalformed
	}
	// Ed25519 的签名输入必须是原始 header.payload 字符串，不能用解码后的 JSON 重组。
	if !ed25519.Verify(publicKey, []byte(parts[0]+"."+parts[1]), signature) {
		return nil, ErrAuthzContextInvalidSignature
	}

	// 签名通过后再解码 payload，避免在未可信数据上做多余业务处理。
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrAuthzContextMalformed
	}

	// 先把 kid 写入上下文，便于后续审计知道使用的是哪把公钥。
	claims := &AuthzContext{KeyId: header.Kid}
	// 将 JSON claim 反序列化为稳定结构，避免业务侧直接操作 map。
	if err := json.Unmarshal(payload, claims); err != nil {
		return nil, ErrAuthzContextMalformed
	}
	// 最后校验 issuer、时间窗口和当前入口期望的资源事实。
	if err := validateAuthzContextClaims(claims, options); err != nil {
		return nil, err
	}
	// 返回已经验签和校验过的可信 authz 上下文。
	return claims, nil
}

func parseAuthzContextHeader(segment string) (*authzContextJWSHeader, error) {
	// JWS header 使用 base64url 编码。
	data, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return nil, ErrAuthzContextMalformed
	}
	// 解析出 alg/kid/typ，后续用 alg 和 kid 做验签约束。
	header := &authzContextJWSHeader{}
	if err := json.Unmarshal(data, header); err != nil {
		return nil, ErrAuthzContextMalformed
	}
	// 返回原始 header 结构，不在这里做业务校验。
	return header, nil
}

func resolveAuthzContextPublicKey(kid string, options AuthzContextVerificationOptions) (ed25519.PublicKey, bool) {
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

func validateAuthzContextClaims(claims *AuthzContext, options AuthzContextVerificationOptions) error {
	// claim 为空说明 payload 没有成功构造成可信上下文。
	if claims == nil {
		return ErrAuthzContextInvalidClaims
	}
	// 配置了 issuer 时必须严格匹配，防止其他签发方 token 混入。
	if options.Issuer != "" && claims.Issuer != options.Issuer {
		return ErrAuthzContextInvalidClaims
	}
	// 主体、目标资源和资源动作是业务服务做上下文判断的最小必要字段。
	if claims.SubjectId == "" || claims.SubjectType == "" || claims.TargetAppId == "" || claims.ResourceType == "" || claims.ResourcePath == "" {
		return ErrAuthzContextInvalidClaims
	}
	// 当前只有 allow 结果会被注入业务服务，其他 decision 一律拒绝。
	if claims.Decision != "" && claims.Decision != "allow" {
		return ErrAuthzContextInvalidClaims
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
		return ErrAuthzContextExpired
	}
	// nbf 可选；写入时表示当前时间必须晚于 nbf。
	if claims.NotBefore > 0 && current.Add(skew).Before(time.Unix(claims.NotBefore, 0)) {
		return ErrAuthzContextNotYetValid
	}
	// 目标 app_id 非空时必须匹配当前服务，避免把 A 服务的授权结果拿到 B 服务复用。
	if options.ExpectedTargetAppId != "" && claims.TargetAppId != options.ExpectedTargetAppId {
		return ErrAuthzContextInvalidClaims
	}
	// 资源动作非空时必须匹配当前入口；gRPC 场景通常是 GRPC。
	if options.ExpectedResourceType != "" && !strings.EqualFold(claims.ResourceType, options.ExpectedResourceType) {
		return ErrAuthzContextInvalidClaims
	}
	// 资源路径非空时必须匹配当前 FullMethod 或 HTTP path。
	if options.ExpectedResourcePath != "" && claims.ResourcePath != options.ExpectedResourcePath {
		return ErrAuthzContextInvalidClaims
	}
	// 所有必要 claim 和本地期望都通过后，认为上下文可信。
	return nil
}
