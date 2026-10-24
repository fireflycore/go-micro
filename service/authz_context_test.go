package service_test

import (
	"crypto/ed25519"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/fireflycore/go-micro/internal/contexttest"
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
			c, err := service.VerifyAuthzSign(contexttest.Sign(contexttest.Claims(tc.subject, tc.user)), contexttest.Options())
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
		"long proof":            func(c map[string]any) { c["exp"] = contexttest.Now.Unix() + 300 },
		"future":                func(c map[string]any) { c["iat"] = contexttest.Now.Unix() + 20; c["exp"] = contexttest.Now.Unix() + 80 },
		"expired":               func(c map[string]any) { c["iat"] = contexttest.Now.Unix() - 60; c["exp"] = contexttest.Now.Unix() },
		"null":                  func(c map[string]any) { c["trace_id"] = nil },
		"wrong time type":       func(c map[string]any) { c["iat"] = "1792627200" },
		"case insensitive JSON": func(c map[string]any) { c["METHOD"] = c["method"]; delete(c, "method") },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			c := contexttest.Claims("service", true)
			mutate(c)
			if _, err := service.VerifyAuthzSign(contexttest.Sign(c), contexttest.Options()); err == nil {
				t.Fatal("accepted invalid claims")
			}
		})
	}
	o := contexttest.Options()
	// 缺少本地实际入口时必须拒绝，不能用证明中的入口补齐本地预期。
	o.ExpectedServicePath = ""
	if _, err := service.VerifyAuthzSign(contexttest.Sign(contexttest.Claims("anonymous", false)), o); err == nil {
		t.Fatal("accepted missing actual entry")
	}
	o = contexttest.Options()
	o.ClockSkew = 6 * time.Second
	if _, err := service.VerifyAuthzSign(contexttest.Sign(contexttest.Claims("anonymous", false)), o); err == nil {
		t.Fatal("accepted unbounded skew")
	}
}

// 受保护头同样使用精确协议，未知 kid/扩展头和错误密钥都不能接受。
func TestSignatureRejectsUntrustedHeaderAndKey(t *testing.T) {
	payload, err := json.Marshal(contexttest.Claims("service", true))
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
		if _, err := service.VerifyAuthzSign(contexttest.SignRawHeader(string(payload), header), contexttest.Options()); err == nil {
			t.Fatal("accepted untrusted header")
		}
	}
	o := contexttest.Options()
	o.PublicKeys = map[string]ed25519.PublicKey{"default": make(ed25519.PublicKey, ed25519.PublicKeySize)}
	if _, err := service.VerifyAuthzSign(contexttest.Sign(contexttest.Claims("service", true)), o); err == nil {
		t.Fatal("accepted wrong key")
	}
}

// 覆盖 compact 格式、非规范 base64url、重复键与直接篡改签名。
func TestSignatureRejectsMalformedAndTampered(t *testing.T) {
	raw := contexttest.Sign(contexttest.Claims("anonymous", false))
	for _, value := range []string{"", "a.b", raw + " ", strings.Replace(raw, ".", "=.", 1), raw[:len(raw)-3] + "abc",
		contexttest.SignRaw(`{"iss":"first","iss":"last"}`)} {
		if _, err := service.VerifyAuthzSign(value, contexttest.Options()); err == nil {
			t.Fatal("accepted malformed/tampered proof")
		}
	}
}
