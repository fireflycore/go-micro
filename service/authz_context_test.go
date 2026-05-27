package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestVerifyAuthzContext(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	now := time.Unix(1710000000, 0).UTC()
	raw := signTestAuthzContext(t, privateKey, "default", map[string]any{
		"iss":           "firefly-authz",
		"sub":           "user-1",
		"subject_type":  "user",
		"user_id":       "user-1",
		"tenant_id":     "tenant-1",
		"invoke_app_id": "app-caller",
		"target_app_id": "app-target",
		"resource_type": "GRPC",
		"path":          "/acme.order.v1.OrderService/List",
		"decision":      "allow",
		"decision_id":   "decision-1",
		"iat":           now.Unix(),
		"exp":           now.Add(time.Minute).Unix(),
	})

	claims, err := VerifyAuthzContext(raw, AuthzContextVerificationOptions{
		PublicKeys:           map[string]ed25519.PublicKey{"default": publicKey},
		Issuer:               "firefly-authz",
		ExpectedTargetAppId:  "app-target",
		ExpectedResourceType: "GRPC",
		ExpectedResourcePath: "/acme.order.v1.OrderService/List",
		Now:                  func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("verify authz context failed: %v", err)
	}
	if claims.KeyId != "default" || claims.SubjectType != "user" || claims.TargetAppId != "app-target" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestVerifyAuthzContext_RejectsInvalidSignature(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate public key failed: %v", err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate signing key failed: %v", err)
	}

	now := time.Unix(1710000000, 0).UTC()
	raw := signTestAuthzContext(t, privateKey, "default", map[string]any{
		"iss":           "firefly-authz",
		"sub":           "user-1",
		"subject_type":  "user",
		"invoke_app_id": "app-caller",
		"target_app_id": "app-target",
		"resource_type": "GRPC",
		"path":          "/acme.order.v1.OrderService/List",
		"decision":      "allow",
		"decision_id":   "decision-1",
		"iat":           now.Unix(),
		"exp":           now.Add(time.Minute).Unix(),
	})

	_, err = VerifyAuthzContext(raw, AuthzContextVerificationOptions{
		PublicKey: publicKey,
		Now:       func() time.Time { return now },
	})
	if !errors.Is(err, ErrAuthzContextInvalidSignature) {
		t.Fatalf("expected invalid signature error, got %v", err)
	}
}

func TestVerifyAuthzContext_RejectsExpiredContext(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	now := time.Unix(1710000000, 0).UTC()
	raw := signTestAuthzContext(t, privateKey, "default", map[string]any{
		"iss":           "firefly-authz",
		"sub":           "user-1",
		"subject_type":  "user",
		"invoke_app_id": "app-caller",
		"target_app_id": "app-target",
		"resource_type": "GRPC",
		"path":          "/acme.order.v1.OrderService/List",
		"decision":      "allow",
		"decision_id":   "decision-1",
		"iat":           now.Add(-2 * time.Minute).Unix(),
		"exp":           now.Add(-time.Minute).Unix(),
	})

	_, err = VerifyAuthzContext(raw, AuthzContextVerificationOptions{
		PublicKey: publicKey,
		Now:       func() time.Time { return now },
	})
	if !errors.Is(err, ErrAuthzContextExpired) {
		t.Fatalf("expected expired error, got %v", err)
	}
}

func signTestAuthzContext(t *testing.T, privateKey ed25519.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()

	header := map[string]any{
		"alg": "EdDSA",
		"kid": kid,
		"typ": "JWT",
	}
	headerSegment := encodeTestJWSSegment(t, header)
	payloadSegment := encodeTestJWSSegment(t, claims)
	signingInput := headerSegment + "." + payloadSegment
	signature := ed25519.Sign(privateKey, []byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func encodeTestJWSSegment(t *testing.T, value any) string {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal jws segment failed: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}
