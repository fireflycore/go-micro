package authz

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewVerificationOptionsDisabled(t *testing.T) {
	// nil 配置表示不启用服务侧验签。
	options, err := NewVerificationOptions(nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if options == nil {
		t.Fatalf("expected non-nil options")
	}
	if options.AuthzVerification != nil {
		t.Fatalf("expected nil authz verification when disabled")
	}
	if len(options.AuthzSkipMethods) != 0 {
		t.Fatalf("expected empty skip methods, got %v", options.AuthzSkipMethods)
	}
}

func TestNewVerificationOptionsLoadsEd25519PublicKey(t *testing.T) {
	// 生成测试用 Ed25519 公钥。
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key failed: %v", err)
	}
	// 写入 PKIX PEM 文件，模拟 openssl pkey -pubout 输出。
	path := writePublicKeyPEM(t, publicKey)

	// 根据配置构造 middleware 验签选项。
	options, err := NewVerificationOptions(&VerificationConfig{
		Enabled:       true,
		Kid:           "default",
		PublicKeyPath: path,
		Issuer:        "firefly-authz",
		ClockSkew:     "3s",
		SkipMethods:   []string{"", "/grpc.health.v1.Health/Check"},
	})
	if err != nil {
		t.Fatalf("build verification options failed: %v", err)
	}
	if options.AuthzVerification == nil {
		t.Fatalf("expected authz verification options")
	}
	if options.AuthzVerification.Issuer != "firefly-authz" {
		t.Fatalf("unexpected issuer: %s", options.AuthzVerification.Issuer)
	}
	if options.AuthzVerification.ClockSkew != 3*time.Second {
		t.Fatalf("unexpected clock skew: %s", options.AuthzVerification.ClockSkew)
	}
	if len(options.AuthzVerification.PublicKeys["default"]) != ed25519.PublicKeySize {
		t.Fatalf("expected default ed25519 public key")
	}
	if len(options.AuthzSkipMethods) != 1 || options.AuthzSkipMethods[0] != "/grpc.health.v1.Health/Check" {
		t.Fatalf("unexpected skip methods: %v", options.AuthzSkipMethods)
	}
}

func TestNewVerificationOptionsUsesDefaults(t *testing.T) {
	// 生成测试用 Ed25519 公钥。
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key failed: %v", err)
	}
	// 写入 PKIX PEM 文件。
	path := writePublicKeyPEM(t, publicKey)

	// 只配置必要字段，验证默认 kid、issuer 和 clock skew。
	options, err := NewVerificationOptions(&VerificationConfig{
		Enabled:       true,
		PublicKeyPath: path,
	})
	if err != nil {
		t.Fatalf("build verification options failed: %v", err)
	}
	if options.AuthzVerification.Issuer != DefaultIssuer {
		t.Fatalf("unexpected default issuer: %s", options.AuthzVerification.Issuer)
	}
	if options.AuthzVerification.ClockSkew != DefaultClockSkew {
		t.Fatalf("unexpected default clock skew: %s", options.AuthzVerification.ClockSkew)
	}
	if len(options.AuthzVerification.PublicKeys[DefaultKid]) != ed25519.PublicKeySize {
		t.Fatalf("expected default kid public key")
	}
}

func TestNewVerificationOptionsRequiresPublicKeyPath(t *testing.T) {
	// 启用验签后 public_key_path 必填。
	_, err := NewVerificationOptions(&VerificationConfig{Enabled: true})
	if err == nil {
		t.Fatalf("expected error when public key path is empty")
	}
}

func TestLoadEd25519PublicKeyRejectsNonEd25519(t *testing.T) {
	// 生成 RSA 公钥，验证算法不匹配时会拒绝。
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key failed: %v", err)
	}
	// 写入 PKIX PEM 文件。
	path := writePublicKeyPEM(t, &privateKey.PublicKey)
	// 加载时应返回算法错误。
	if _, err := LoadEd25519PublicKey(path); err == nil {
		t.Fatalf("expected non-ed25519 public key error")
	}
}

func writePublicKeyPEM(t *testing.T, publicKey any) string {
	// 标记测试 helper，失败时显示调用方行号。
	t.Helper()
	// 将公钥编码为 PKIX DER。
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("marshal public key failed: %v", err)
	}
	// 生成临时 PEM 文件路径。
	path := filepath.Join(t.TempDir(), "public-key.pem")
	// 写入 PEM 公钥。
	data := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write public key failed: %v", err)
	}
	// 返回 PEM 文件路径。
	return path
}
