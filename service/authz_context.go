package service

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fireflycore/go-micro/constant"
)

var (
	// ErrAuthzSignMissing 表示入口要求验签，但未收到本跳证明。
	ErrAuthzSignMissing = errors.New("authz sign is missing")
	// ErrAuthzSignMalformed 表示 compact JWS 或 JSON 编码不符合协议。
	ErrAuthzSignMalformed = errors.New("authz sign is malformed")
	// ErrAuthzSignPublicKeyMissing 表示 kid 无匹配密钥或公钥长度不正确。
	ErrAuthzSignPublicKeyMissing = errors.New("authz sign public key is missing")
	// ErrAuthzSignUnsupportedAlg 表示签名算法不是固定的 EdDSA。
	ErrAuthzSignUnsupportedAlg = errors.New("authz sign alg is unsupported")
	// ErrAuthzSignInvalidSignature 表示签名与原始受保护内容不匹配。
	ErrAuthzSignInvalidSignature = errors.New("authz sign signature is invalid")
	// ErrAuthzSignInvalidClaims 表示新 claims、主体或本地投递绑定校验失败。
	ErrAuthzSignInvalidClaims = errors.New("authz sign claims are invalid")
	// ErrAuthzSignExpired 表示证明已超过允许的有效期。
	ErrAuthzSignExpired = errors.New("authz sign is expired")
	// ErrAuthzSignNotYetValid 表示签发时间或生效时间仍在未来。
	ErrAuthzSignNotYetValid = errors.New("authz sign is not yet valid")
)

// AuthzSignContext 是验签后的授权投影，不保存 compact JWS 原文。
type AuthzSignContext struct {
	// KeyId 来自受保护头 kid，不是 payload claim。
	KeyId string `json:"-"`
	// Issuer 是签发方 iss，启用配置时须与本地预期一致。
	Issuer string `json:"iss"`
	// SubjectId 是本跳授权主体标识，不等同于用户应用。
	SubjectId string `json:"sub"`
	// SubjectType 只允许 user、service 或 anonymous。
	SubjectType string `json:"subject_type"`
	// SubjectAppId 是实际授权主体应用，必须与对应用户或服务身份一致。
	SubjectAppId string `json:"subject_app_id,omitempty"`
	// InvokeServiceAppId 是本跳已认证调用服务；运行时 InvokeAppId 只从此派生。
	InvokeServiceAppId string `json:"invoke_service_app_id,omitempty"`
	// Method 表示授权 API 的动作，不是实际投递方法。
	Method string `json:"method"`
	// Path 表示授权 API 的路径，HTTP 转 gRPC 时可与 ServicePath 不同。
	Path string `json:"path"`
	// ServiceAppId 绑定允许投递的服务，必须与本地配置比较。
	ServiceAppId string `json:"service_app_id"`
	// ServiceMethod 绑定实际投递方法，不能用 Method 补齐。
	ServiceMethod string `json:"service_method"`
	// ServicePath 绑定实际投递入口，必须与适配器捕获值一致。
	ServicePath string `json:"service_path"`
	// Decision 只接受 allow，拒绝结果不得构建业务上下文。
	Decision string `json:"decision"`
	// DecisionId 是本跳决策标识，用于审计与日志关联。
	DecisionId string `json:"decision_id"`
	// IssuedAt 是 Unix 秒级签发时间。
	IssuedAt int64 `json:"iat"`
	// NotBefore 是可选的 Unix 秒级生效时间，零表示未指定。
	NotBefore int64 `json:"nbf,omitempty"`
	// ExpiresAt 是 Unix 秒级过期时间，必须恰好比 IssuedAt 晚 60 秒。
	ExpiresAt int64 `json:"exp"`
}

// AuthzSign 是传输 payload；公开运行时只保留所需投影，不保存签名串。
type AuthzSign struct {
	// AuthzSignContext 承载本跳主体、授权 API 与独立投递绑定。
	AuthzSignContext
	// UserContext 独立保留已验证用户；服务主体不意味着丢弃用户视图。
	UserContext *UserContext `json:"user_context,omitempty"`
	// TraceId 仅供签发侧观测，不替代当前 OTel 活动 span。
	TraceId string `json:"trace_id,omitempty"`
}

// AuthzSignVerificationOptions 将本地可信事实与签名提供的声明分开。
type AuthzSignVerificationOptions struct {
	// PublicKey 用于单公钥配置；配置 PublicKeys 时按 kid 映射选择。
	PublicKey ed25519.PublicKey
	// PublicKeys 是 kid 到公钥的映射，用于显式密钥选择和轮换。
	PublicKeys map[string]ed25519.PublicKey
	// Issuer 非空时限制签发方，配置装配层通常设置框架默认值。
	Issuer string
	// ExpectedMethod 是可选授权动作约束，不能替代实际投递方法。
	ExpectedMethod string
	// ExpectedPath 是可选授权路径约束，不能替代实际入口。
	ExpectedPath string
	// ExpectedServiceAppId 必填，来自本地服务配置。
	ExpectedServiceAppId string
	// ExpectedServiceMethod 必填，来自适配器捕获的真实入口方法。
	ExpectedServiceMethod string
	// ExpectedServicePath 必填，来自适配器捕获的真实入口路径。
	ExpectedServicePath string
	// ClockSkew 只允许 0 到 5 秒，不能借此扩展为长时效证明。
	ClockSkew time.Duration
	// Now 用于注入测试时钟；未指定时使用当前系统时间。
	Now func() time.Time
}

// authzSignJWSHeader 只接受协议明确列出的受保护头，不自动接受扩展字段。
type authzSignJWSHeader struct {
	// Alg 固定为 EdDSA。
	Alg string `json:"alg"`
	// Kid 必须非空，并用于公钥映射选择。
	Kid string `json:"kid"`
	// Typ 固定为 JWT。
	Typ string `json:"typ"`
}

// VerifyAuthzSign 严格验证本跳证明；原生 gRPC 也必须提供独立的本地投递绑定。
// 授权 Method/Path 不得补齐缺失的 Service* 字段，不兼容旧 claims。
func VerifyAuthzSign(raw string, options AuthzSignVerificationOptions) (*AuthzSign, error) {
	if raw == "" {
		return nil, ErrAuthzSignMissing
	}
	if len(raw) > 16384 {
		return nil, ErrAuthzSignMalformed
	}

	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, ErrAuthzSignMalformed
	}

	decode := func(part string) ([]byte, error) {
		// compact JWS 使用无填充的规范 base64url，拒绝空片段、空白与等号。
		if part == "" || strings.ContainsAny(part, "= \t\r\n") {
			return nil, ErrAuthzSignMalformed
		}

		b, err := base64.RawURLEncoding.Strict().DecodeString(part)
		if err != nil {
			return nil, ErrAuthzSignMalformed
		}

		return b, nil
	}

	head, err := decode(parts[0])
	if err != nil {
		return nil, err
	}

	var header authzSignJWSHeader
	if strictJSON(head, &header) != nil {
		return nil, ErrAuthzSignMalformed
	}
	if header.Alg != constant.JWSAlgorithmEdDSA {
		return nil, ErrAuthzSignUnsupportedAlg
	}
	if header.Typ != constant.JWSTypeJWT || header.Kid == "" {
		return nil, ErrAuthzSignMalformed
	}

	key := options.PublicKey
	if len(options.PublicKeys) > 0 {
		// 启用 kid 映射后不回退单公钥，未知 kid 必须失败。
		key = options.PublicKeys[header.Kid]
	}
	if len(key) != ed25519.PublicKeySize {
		return nil, ErrAuthzSignPublicKeyMissing
	}

	sig, err := decode(parts[2])
	if err != nil {
		return nil, err
	}

	// 先验证受保护头与原始 payload 片段，之后才解释 payload 的身份声明。
	if !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), sig) {
		return nil, ErrAuthzSignInvalidSignature
	}

	payload, err := decode(parts[1])
	if err != nil {
		return nil, err
	}

	var claims AuthzSign
	if strictJSON(payload, &claims) != nil {
		return nil, ErrAuthzSignInvalidClaims
	}

	claims.KeyId = header.Kid
	if err := validateAuthzSignClaims(&claims, options); err != nil {
		return nil, err
	}

	return &claims, nil
}

// validateAuthzSignClaims 依次约束投递绑定、主体一致性与固定短时效。
func validateAuthzSignClaims(c *AuthzSign, o AuthzSignVerificationOptions) error {
	if o.ExpectedServiceAppId == "" || o.ExpectedServiceMethod == "" || o.ExpectedServicePath == "" ||
		c.Issuer == "" || c.SubjectId == "" || c.Method == "" || c.Path == "" || c.DecisionId == "" ||
		c.ServiceAppId != o.ExpectedServiceAppId || c.ServiceMethod != o.ExpectedServiceMethod || c.ServicePath != o.ExpectedServicePath ||
		c.Decision != constant.AuthzSignDecisionAllow || (o.Issuer != "" && c.Issuer != o.Issuer) ||
		(o.ExpectedMethod != "" && c.Method != o.ExpectedMethod) || (o.ExpectedPath != "" && c.Path != o.ExpectedPath) {
		return ErrAuthzSignInvalidClaims
	}

	if c.UserContext != nil && (c.UserContext.UserId == "" || c.UserContext.AppId == "" || c.UserContext.Session == "") {
		return ErrAuthzSignInvalidClaims
	}

	switch c.SubjectType {
	case constant.SubjectTypeUser:
		// 用户直调只能按用户登录应用授权，不能同时伪造上一跳服务。
		if c.UserContext == nil || c.SubjectAppId != c.UserContext.AppId || c.InvokeServiceAppId != "" {
			return ErrAuthzSignInvalidClaims
		}
	case constant.SubjectTypeService:
		// 服务调用以已认证服务为主体；附带用户视图仍可独立保留。
		if c.InvokeServiceAppId == "" || c.SubjectAppId != c.InvokeServiceAppId {
			return ErrAuthzSignInvalidClaims
		}
	case constant.SubjectTypeAnonymous:
		// 匿名证明不能携带伪造应用或用户身份。
		if c.SubjectAppId != "" || c.InvokeServiceAppId != "" || c.UserContext != nil {
			return ErrAuthzSignInvalidClaims
		}
	default:
		return ErrAuthzSignInvalidClaims
	}

	if o.ClockSkew < 0 || o.ClockSkew > 5*time.Second || c.IssuedAt <= 0 || c.ExpiresAt-c.IssuedAt != 60 ||
		c.NotBefore < 0 || c.NotBefore > c.ExpiresAt {
		return ErrAuthzSignInvalidClaims
	}

	now := time.Now()
	if o.Now != nil {
		now = o.Now()
	}

	// 到达过期边界即拒绝；时钟偏差只用于跨节点容差，不改变声明时长。
	if !now.Add(-o.ClockSkew).Before(time.Unix(c.ExpiresAt, 0)) {
		return ErrAuthzSignExpired
	}
	if now.Add(o.ClockSkew).Before(time.Unix(c.IssuedAt, 0)) ||
		(c.NotBefore > 0 && now.Add(o.ClockSkew).Before(time.Unix(c.NotBefore, 0))) {
		return ErrAuthzSignNotYetValid
	}

	return nil
}

// strictJSON 拒绝未知/旧字段、非法 null、大小写变体和重复键，统一跨语言解释。
func strictJSON(data []byte, out any) error {
	if !utf8.Valid(data) {
		return ErrAuthzSignMalformed
	}

	allowed := map[string]bool{}
	switch out.(type) {
	case *authzSignJWSHeader:
		for _, k := range []string{"alg", "kid", "typ"} {
			allowed[k] = true
		}
	case *AuthzSign:
		for _, k := range []string{"iss", "sub", "subject_type", "subject_app_id", "invoke_service_app_id",
			"method", "path", "service_app_id", "service_method", "service_path", "decision", "decision_id",
			"iat", "nbf", "exp", "user_context", "trace_id"} {
			allowed[k] = true
		}
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return ErrAuthzSignMalformed
	}
	for k, v := range fields {
		// 精确匹配字段名，避免 encoding/json 的大小写宽松匹配形成旁路。
		if !allowed[k] || (string(v) == "null" && k != "user_context") {
			return ErrAuthzSignMalformed
		}
		if k == "user_context" && string(v) != "null" {
			var user map[string]json.RawMessage
			if json.Unmarshal(v, &user) != nil || user == nil {
				return ErrAuthzSignMalformed
			}
			for uk, uv := range user {
				if (uk != "session" && uk != "user_id" && uk != "app_id" && uk != "tenant_id") || string(uv) == "null" {
					return ErrAuthzSignMalformed
				}
			}
		}
	}

	d := json.NewDecoder(bytes.NewReader(data))

	// 逐层读取 token 检查重复键，普通 map 反序列化会静默覆盖它们。
	var walk func() error
	walk = func() error {
		t, err := d.Token()
		if err != nil {
			return err
		}
		if delim, ok := t.(json.Delim); ok {
			if delim != '{' && delim != '[' {
				return ErrAuthzSignMalformed
			}
			seen := map[string]bool{}
			for d.More() {
				if delim == '{' {
					k, ke := d.Token()
					if ke != nil {
						return ke
					}
					key, ok := k.(string)
					if !ok || seen[key] {
						return ErrAuthzSignMalformed
					}
					seen[key] = true
				}
				if we := walk(); we != nil {
					return we
				}
			}
			_, err = d.Token()
			return err
		}
		return nil
	}
	if err := walk(); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		// 只接受单个完整 JSON 值，不接受对象后的额外内容。
		return ErrAuthzSignMalformed
	}

	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()

	return d.Decode(out)
}
