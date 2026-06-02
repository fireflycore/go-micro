package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/fireflycore/go-micro/constant"
)

const (
	testAuthzKid           = "default"
	testAuthzIssuer        = "firefly-authz"
	testAuthzDecisionAllow = "allow"
)

func TestVerifyAuthzSign(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	now := time.Unix(1710000000, 0).UTC()
	raw := signTestAuthzSign(t, privateKey, testAuthzKid, map[string]any{
		"iss":           testAuthzIssuer,
		"sub":           "user-1",
		"subject_type":  constant.SubjectTypeUser,
		"user_id":       "user-1",
		"app_id":        "user-app",
		"session":       "session-1",
		"tenant_id":     "tenant-1",
		"invoke_app_id": "app-caller",
		"target_app_id": "app-target",
		"api_method":    constant.RequestMethodGrpcString,
		"api_path":      "/acme.order.v1.OrderService/List",
		"decision":      testAuthzDecisionAllow,
		"decision_id":   "decision-1",
		"iat":           now.Unix(),
		"exp":           now.Add(time.Minute).Unix(),
	})

	claims, err := VerifyAuthzSign(raw, AuthzSignVerificationOptions{
		PublicKeys:          map[string]ed25519.PublicKey{testAuthzKid: publicKey},
		Issuer:              testAuthzIssuer,
		ExpectedTargetAppId: "app-target",
		ExpectedApiMethod:   constant.RequestMethodGrpcString,
		ExpectedApiPath:     "/acme.order.v1.OrderService/List",
		Now:                 func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("verify authz sign failed: %v", err)
	}
	if claims.KeyId != testAuthzKid || claims.SubjectType != constant.SubjectTypeUser || claims.TargetAppId != "app-target" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.Session != "session-1" {
		t.Fatalf("unexpected session claim: %+v", claims)
	}
	if claims.AppId != "user-app" {
		t.Fatalf("unexpected user app id claim: %+v", claims)
	}
}

func TestVerifyAuthzSign_NormalizesStructuredClaims(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	now := time.Unix(1710000000, 0).UTC()
	raw := signTestAuthzSign(t, privateKey, testAuthzKid, map[string]any{
		"iss":          testAuthzIssuer,
		"sub":          "service-a",
		"subject_type": constant.SubjectTypeService,
		"user_context": map[string]any{
			"user_id":   "user-1",
			"app_id":    "user-app",
			"tenant_id": "tenant-1",
			"session":   "session-1",
			"org_ids":   []string{"org-1"},
			"post_ids":  []string{"post-1"},
			"role_ids":  []string{"role-1"},
		},
		"invoke_service_context": map[string]any{
			"app_id":      "service-a",
			"instance_id": "service-a-1",
		},
		"target_service_context": map[string]any{
			"app_id":      "svc-app",
			"instance_id": "svc-app-1",
		},
		"api_method":  constant.RequestMethodGrpcString,
		"api_path":    "/acme.test.v1.TestService/Get",
		"decision":    testAuthzDecisionAllow,
		"decision_id": "decision-1",
		"iat":         now.Unix(),
		"exp":         now.Add(time.Minute).Unix(),
	})

	claims, err := VerifyAuthzSign(raw, AuthzSignVerificationOptions{
		PublicKeys:          map[string]ed25519.PublicKey{testAuthzKid: publicKey},
		Issuer:              testAuthzIssuer,
		ExpectedTargetAppId: "svc-app",
		ExpectedApiMethod:   constant.RequestMethodGrpcString,
		ExpectedApiPath:     "/acme.test.v1.TestService/Get",
		Now:                 func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("verify authz sign failed: %v", err)
	}
	if claims.AppId != "user-app" || claims.UserId != "user-1" || claims.TargetAppId != "svc-app" {
		t.Fatalf("expected structured claims to be normalized: %+v", claims)
	}
	if claims.InvokeAppId != "service-a" || claims.InvokeInstanceId != "service-a-1" {
		t.Fatalf("expected invoke service context to be normalized: %+v", claims)
	}
	if claims.InvokeServiceContext == nil || claims.InvokeServiceContext.AppId != "service-a" {
		t.Fatalf("expected service context claim: %+v", claims)
	}
	if claims.TargetServiceContext == nil || claims.TargetServiceContext.AppId != "svc-app" || claims.TargetInstanceId != "svc-app-1" {
		t.Fatalf("expected route context claim: %+v", claims)
	}
	if claims.Session != "session-1" || len(claims.OrgIds) != 1 || len(claims.PostIds) != 1 || len(claims.RoleIds) != 1 {
		t.Fatalf("expected user context scope claims to be normalized: %+v", claims)
	}
}

func TestVerifyAuthzSign_RejectsInvalidSignature(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate public key failed: %v", err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate signing key failed: %v", err)
	}

	now := time.Unix(1710000000, 0).UTC()
	raw := signTestAuthzSign(t, privateKey, testAuthzKid, map[string]any{
		"iss":           testAuthzIssuer,
		"sub":           "user-1",
		"subject_type":  constant.SubjectTypeUser,
		"invoke_app_id": "app-caller",
		"target_app_id": "app-target",
		"api_method":    constant.RequestMethodGrpcString,
		"api_path":      "/acme.order.v1.OrderService/List",
		"decision":      testAuthzDecisionAllow,
		"decision_id":   "decision-1",
		"iat":           now.Unix(),
		"exp":           now.Add(time.Minute).Unix(),
	})

	_, err = VerifyAuthzSign(raw, AuthzSignVerificationOptions{
		PublicKey: publicKey,
		Now:       func() time.Time { return now },
	})
	if !errors.Is(err, ErrAuthzSignInvalidSignature) {
		t.Fatalf("expected invalid signature error, got %v", err)
	}
}

func TestVerifyAuthzSign_RejectsExpiredContext(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	now := time.Unix(1710000000, 0).UTC()
	raw := signTestAuthzSign(t, privateKey, testAuthzKid, map[string]any{
		"iss":           testAuthzIssuer,
		"sub":           "user-1",
		"subject_type":  constant.SubjectTypeUser,
		"invoke_app_id": "app-caller",
		"target_app_id": "app-target",
		"api_method":    constant.RequestMethodGrpcString,
		"api_path":      "/acme.order.v1.OrderService/List",
		"decision":      testAuthzDecisionAllow,
		"decision_id":   "decision-1",
		"iat":           now.Add(-2 * time.Minute).Unix(),
		"exp":           now.Add(-time.Minute).Unix(),
	})

	_, err = VerifyAuthzSign(raw, AuthzSignVerificationOptions{
		PublicKey: publicKey,
		Now:       func() time.Time { return now },
	})
	if !errors.Is(err, ErrAuthzSignExpired) {
		t.Fatalf("expected expired error, got %v", err)
	}
}

func TestVerifyAuthzSign_RejectsMissingMethod(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	now := time.Unix(1710000000, 0).UTC()
	raw := signTestAuthzSign(t, privateKey, testAuthzKid, map[string]any{
		"iss":           testAuthzIssuer,
		"sub":           "user-1",
		"subject_type":  constant.SubjectTypeUser,
		"invoke_app_id": "app-caller",
		"target_app_id": "app-target",
		"resource_type": constant.RequestMethodGrpcString,
		"api_path":      "/acme.order.v1.OrderService/List",
		"decision":      testAuthzDecisionAllow,
		"decision_id":   "decision-1",
		"iat":           now.Unix(),
		"exp":           now.Add(time.Minute).Unix(),
	})

	_, err = VerifyAuthzSign(raw, AuthzSignVerificationOptions{
		PublicKeys: map[string]ed25519.PublicKey{testAuthzKid: publicKey},
		Now:        func() time.Time { return now },
	})
	if !errors.Is(err, ErrAuthzSignInvalidClaims) {
		t.Fatalf("expected invalid claims for removed resource_type, got %v", err)
	}
}

func signTestAuthzSign(t *testing.T, privateKey ed25519.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()

	header := map[string]any{
		"alg": constant.JWSAlgorithmEdDSA,
		"kid": kid,
		"typ": constant.JWSTypeJWT,
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
