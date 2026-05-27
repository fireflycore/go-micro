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
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

// VerifyAuthzContext 校验 authz 签名上下文并返回可信 claim。
func VerifyAuthzContext(raw string, options AuthzContextVerificationOptions) (*AuthzContext, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrAuthzContextMissing
	}

	parts := strings.Split(raw, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, ErrAuthzContextMalformed
	}

	header, err := parseAuthzContextHeader(parts[0])
	if err != nil {
		return nil, err
	}
	if header.Alg != "EdDSA" {
		return nil, ErrAuthzContextUnsupportedAlg
	}

	publicKey, ok := resolveAuthzContextPublicKey(header.Kid, options)
	if !ok {
		return nil, ErrAuthzContextPublicKeyMissing
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrAuthzContextMalformed
	}
	if !ed25519.Verify(publicKey, []byte(parts[0]+"."+parts[1]), signature) {
		return nil, ErrAuthzContextInvalidSignature
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrAuthzContextMalformed
	}

	claims := &AuthzContext{KeyId: header.Kid}
	if err := json.Unmarshal(payload, claims); err != nil {
		return nil, ErrAuthzContextMalformed
	}
	if err := validateAuthzContextClaims(claims, options); err != nil {
		return nil, err
	}
	return claims, nil
}

func parseAuthzContextHeader(segment string) (*authzContextJWSHeader, error) {
	data, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return nil, ErrAuthzContextMalformed
	}
	header := &authzContextJWSHeader{}
	if err := json.Unmarshal(data, header); err != nil {
		return nil, ErrAuthzContextMalformed
	}
	return header, nil
}

func resolveAuthzContextPublicKey(kid string, options AuthzContextVerificationOptions) (ed25519.PublicKey, bool) {
	if len(options.PublicKeys) > 0 {
		key, ok := options.PublicKeys[kid]
		return key, ok && len(key) == ed25519.PublicKeySize
	}
	if len(options.PublicKey) == ed25519.PublicKeySize {
		return options.PublicKey, true
	}
	return nil, false
}

func validateAuthzContextClaims(claims *AuthzContext, options AuthzContextVerificationOptions) error {
	if claims == nil {
		return ErrAuthzContextInvalidClaims
	}
	if options.Issuer != "" && claims.Issuer != options.Issuer {
		return ErrAuthzContextInvalidClaims
	}
	if claims.SubjectId == "" || claims.SubjectType == "" || claims.TargetAppId == "" || claims.ResourceType == "" || claims.ResourcePath == "" {
		return ErrAuthzContextInvalidClaims
	}
	if claims.Decision != "" && claims.Decision != "allow" {
		return ErrAuthzContextInvalidClaims
	}

	now := time.Now
	if options.Now != nil {
		now = options.Now
	}
	current := now().UTC()
	skew := options.ClockSkew
	if claims.ExpiresAt <= 0 || !current.Add(-skew).Before(time.Unix(claims.ExpiresAt, 0)) {
		return ErrAuthzContextExpired
	}
	if claims.NotBefore > 0 && current.Add(skew).Before(time.Unix(claims.NotBefore, 0)) {
		return ErrAuthzContextNotYetValid
	}
	if options.ExpectedTargetAppId != "" && claims.TargetAppId != options.ExpectedTargetAppId {
		return ErrAuthzContextInvalidClaims
	}
	if options.ExpectedResourceType != "" && !strings.EqualFold(claims.ResourceType, options.ExpectedResourceType) {
		return ErrAuthzContextInvalidClaims
	}
	if options.ExpectedResourcePath != "" && claims.ResourcePath != options.ExpectedResourcePath {
		return ErrAuthzContextInvalidClaims
	}
	return nil
}
