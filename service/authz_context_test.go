package service_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/fireflycore/go-micro/service"
)

// 验证匿名、用户、纯服务和双身份四种合法投影，不以服务主体覆盖用户。
func TestSignatureSubjects(t *testing.T) {
	for _, tc := range []struct {
		// subject 是本跳授权主体类型。
		subject string
		// user 表示是否同时携带独立的已验证用户视图。
		user bool
	}{
		{"anonymous", false}, {"user", true}, {"service", false}, {"service", true},
	} {
		t.Run(tc.subject+string(rune(boolInt(tc.user)+'0')), func(t *testing.T) {
			c, err := service.VerifyAuthzSign(authzTestSign(t, authzTestClaims(tc.subject, tc.user)), authzTestOptions())
			if err != nil {
				t.Fatal(err)
			}
			if (c.UserContext != nil) != tc.user || c.SubjectType != tc.subject {
				t.Fatal("wrong projection")
			}
		})
	}
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// 每个负例都重新签名，确认旧 claims、主体不一致和错误绑定不能通过有效签名绕过。
func TestSignatureRejectsInvalidContract(t *testing.T) {
	mutations := map[string]func(map[string]any){
		"old claims":            func(c map[string]any) { c["route_method"] = "POST" },
		"old user collections":  func(c map[string]any) { c["user_context"].(map[string]any)["org_ids"] = []string{"org"} },
		"wrong subject":         func(c map[string]any) { c["subject_app_id"] = "login-app" },
		"wrong issuer":          func(c map[string]any) { c["iss"] = "other" },
		"unknown subject":       func(c map[string]any) { c["subject_type"] = "other" },
		"wrong app":             func(c map[string]any) { c["service_app_id"] = "C" },
		"wrong actual protocol": func(c map[string]any) { c["service_method"] = "POST" },
		"wrong actual path":     func(c map[string]any) { c["service_path"] = "/other" },
		"missing binding":       func(c map[string]any) { delete(c, "service_path") },
		"no fallback":           func(c map[string]any) { delete(c, "service_method"); c["method"] = "GRPC" },
		"long proof":            func(c map[string]any) { c["exp"] = authzTestNow.Unix() + 300 },
		"future":                func(c map[string]any) { c["iat"] = authzTestNow.Unix() + 20; c["exp"] = authzTestNow.Unix() + 80 },
		"expired":               func(c map[string]any) { c["iat"] = authzTestNow.Unix() - 60; c["exp"] = authzTestNow.Unix() },
		"null":                  func(c map[string]any) { c["trace_id"] = nil },
		"wrong time type":       func(c map[string]any) { c["iat"] = "1792627200" },
		"case insensitive JSON": func(c map[string]any) { c["METHOD"] = c["method"]; delete(c, "method") },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			c := authzTestClaims("service", true)
			mutate(c)
			if _, err := service.VerifyAuthzSign(authzTestSign(t, c), authzTestOptions()); err == nil {
				t.Fatal("accepted invalid claims")
			}
		})
	}
	o := authzTestOptions()
	// 缺少本地实际入口时必须拒绝，不能用证明中的入口补齐本地预期。
	o.ExpectedServicePath = ""
	if _, err := service.VerifyAuthzSign(authzTestSign(t, authzTestClaims("anonymous", false)), o); err == nil {
		t.Fatal("accepted missing actual entry")
	}
	o = authzTestOptions()
	o.ClockSkew = 6 * time.Second
	if _, err := service.VerifyAuthzSign(authzTestSign(t, authzTestClaims("anonymous", false)), o); err == nil {
		t.Fatal("accepted unbounded skew")
	}
}

// 受保护头同样使用精确协议，未知 kid/扩展头和错误密钥都不能接受。
func TestSignatureRejectsUntrustedHeaderAndKey(t *testing.T) {
	payload, err := json.Marshal(authzTestClaims("service", true))
	if err != nil {
		t.Fatal(err)
	}
	for _, header := range []string{
		`{"alg":"HS256","kid":"default","typ":"JWT"}`,
		`{"alg":"EdDSA","kid":"unknown","typ":"JWT"}`,
		`{"alg":"EdDSA","kid":"","typ":"JWT"}`,
		`{"alg":"EdDSA","kid":"default","typ":"other"}`,
		`{"alg":"EdDSA","kid":"default","typ":"JWT","jku":"https://untrusted.invalid"}`,
		`{"alg":"EdDSA","kid":"default","kid":"other","typ":"JWT"}`,
	} {
		if _, err := service.VerifyAuthzSign(authzTestSignRawHeader(string(payload), header), authzTestOptions()); err == nil {
			t.Fatal("accepted untrusted header")
		}
	}
	o := authzTestOptions()
	o.PublicKeys = map[string]ed25519.PublicKey{"default": make(ed25519.PublicKey, ed25519.PublicKeySize)}
	if _, err := service.VerifyAuthzSign(authzTestSign(t, authzTestClaims("service", true)), o); err == nil {
		t.Fatal("accepted wrong key")
	}
}

// 覆盖 compact 格式、非规范 base64url、重复键与直接篡改签名。
func TestSignatureRejectsMalformedAndTampered(t *testing.T) {
	raw := authzTestSign(t, authzTestClaims("anonymous", false))
	for _, value := range []string{"", "a.b", raw + " ", strings.Replace(raw, ".", "=.", 1), raw[:len(raw)-3] + "abc",
		authzTestSignRaw(`{"iss":"first","iss":"last"}`)} {
		if _, err := service.VerifyAuthzSign(value, authzTestOptions()); err == nil {
			t.Fatal("accepted malformed/tampered proof")
		}
	}
}

// authzTestNow 固定签名契约测试的时钟，不依赖运行测试时的系统时间。
var authzTestNow = time.Unix(1792627200, 0)

// authzTestKey 仅供本包测试签名使用，全零种子绝不能用于实际服务。
var authzTestKey = ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))

// authzTestClaims 构造四种主体/用户组合，授权 HTTP 入口与实际 gRPC 投递故意不同。
func authzTestClaims(subject string, user bool) map[string]any {
	c := map[string]any{
		"iss": "firefly-authz", "sub": "anonymous", "subject_type": subject,
		"method": "POST", "path": "/v1/items", "service_app_id": "B",
		"service_method": "GRPC", "service_path": "/items.Service/Get",
		"decision": "allow", "decision_id": "decision", "iat": authzTestNow.Unix(), "exp": authzTestNow.Unix() + 60,
	}
	if user {
		// 用户租户独立保留，不与服务主体会话或租户建立隐式关联。
		c["user_context"] = map[string]any{"session": "secret-session", "user_id": "user", "app_id": "login-app", "tenant_id": "user-tenant"}
	}
	switch subject {
	case "user":
		c["sub"], c["subject_app_id"] = "user", "login-app"
	case "service":
		c["sub"], c["subject_app_id"], c["invoke_service_app_id"] = "A", "A", "A"
	}
	return c
}

// authzTestSign 允许负例修改声明后重新签名，区分密码学有效与契约合法。
func authzTestSign(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	return authzTestSignRaw(string(payload))
}

// authzTestSignRaw 保留原始 JSON，用于重复键、大小写和非法字段负例。
func authzTestSignRaw(payload string) string {
	return authzTestSignRawHeader(payload, `{"alg":"EdDSA","kid":"default","typ":"JWT"}`)
}

// authzTestSignRawHeader 允许替换受保护头，验证算法、kid 和类型拒绝规则。
func authzTestSignRawHeader(payload, header string) string {
	enc := base64.RawURLEncoding.EncodeToString
	input := enc([]byte(header)) + "." + enc([]byte(payload))
	return input + "." + enc(ed25519.Sign(authzTestKey, []byte(input)))
}

// authzTestOptions 固定本地投递绑定，不从待验声明生成预期服务或入口。
func authzTestOptions() service.AuthzSignVerificationOptions {
	return service.AuthzSignVerificationOptions{
		PublicKeys: map[string]ed25519.PublicKey{"default": authzTestKey.Public().(ed25519.PublicKey)}, Issuer: "firefly-authz",
		ExpectedServiceAppId: "B", ExpectedServiceMethod: "GRPC", ExpectedServicePath: "/items.Service/Get",
		Now: func() time.Time { return authzTestNow },
	}
}
