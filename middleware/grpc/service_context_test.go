package gm

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/fireflycore/go-micro/constant"
	servicectx "github.com/fireflycore/go-micro/service"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	testAuthzKid           = "default"
	testAuthzIssuer        = "firefly-authz"
	testAuthzDecisionAllow = "allow"
)

func TestNewServiceContextUnaryInterceptor(t *testing.T) {
	original := otel.GetTracerProvider()
	provider := trace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	defer func() {
		otel.SetTracerProvider(original)
		_ = provider.Shutdown(context.Background())
	}()

	tracer := provider.Tracer("gm-test")
	baseCtx, span := tracer.Start(context.Background(), "interceptor")
	defer span.End()

	baseCtx = metadata.NewIncomingContext(baseCtx, metadata.Pairs(
		constant.UserId, "user-1",
		constant.AppId, "app-1",
		constant.TenantId, "tenant-1",
		constant.OrgIds, "org-1",
		constant.PostIds, "post-1",
		constant.RoleIds, "role-1",
		constant.SubjectType, constant.SubjectTypeUser,
		constant.InvokeAppId, "app-1",
		constant.TargetAppId, "svc-app",
	))

	interceptor := NewServiceContextUnaryInterceptor(ServiceContextInterceptorOptions{
		ExpectedTargetAppId: "svc-app",
	})

	resp, err := interceptor(baseCtx, &struct{}{}, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		value, ok := servicectx.FromContext(ctx)
		if !ok {
			t.Fatal("expected service context in handler context")
		}
		if value.UserId != "user-1" || value.AppId != "app-1" {
			t.Fatalf("unexpected service context: %+v", value)
		}
		if value.SubjectType != constant.SubjectTypeUser || value.TargetAppId != "svc-app" {
			t.Fatalf("unexpected authz fields: %+v", value)
		}
		if value.TraceId == "" {
			t.Fatal("expected trace id from active span")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}
}

func TestNewServiceContextUnaryInterceptor_VerifiesAuthzSign(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	now := time.Unix(1710000000, 0).UTC()
	token := signTestAuthzSign(t, privateKey, testAuthzKid, map[string]any{
		"iss":          testAuthzIssuer,
		"sub":          "user-1",
		"subject_type": constant.SubjectTypeUser,
		"user_id":      "user-1",
		"app_id":       "user-app",
		"tenant_id":    "tenant-1",
		"user_context": map[string]any{
			"user_id":   "user-1",
			"app_id":    "user-app",
			"tenant_id": "tenant-1",
			"post_ids":  []string{"post-1"},
		},
		"invoke_app_id": "app-caller",
		"target_app_id": "svc-app",
		"api_method":    constant.RequestMethodGrpcString,
		"api_path":      "/acme.test.v1.TestService/Get",
		"decision":      testAuthzDecisionAllow,
		"decision_id":   "decision-1",
		"iat":           now.Unix(),
		"exp":           now.Add(time.Minute).Unix(),
	})

	baseCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.AuthzSign, token,
	))
	interceptor := NewServiceContextUnaryInterceptor(ServiceContextInterceptorOptions{
		ExpectedTargetAppId: "svc-app",
		AuthzVerification: &servicectx.AuthzSignVerificationOptions{
			PublicKeys: map[string]ed25519.PublicKey{testAuthzKid: publicKey},
			Issuer:     testAuthzIssuer,
			Now:        func() time.Time { return now },
		},
	})

	resp, err := interceptor(baseCtx, &struct{}{}, &grpc.UnaryServerInfo{FullMethod: "/acme.test.v1.TestService/Get"}, func(ctx context.Context, req any) (any, error) {
		value, ok := servicectx.FromContext(ctx)
		if !ok {
			t.Fatal("expected service context in handler context")
		}
		if value.VerifiedAuthzSign == nil || value.UserId != "user-1" || value.AppId != "user-app" || value.InvokeAppId != "app-caller" {
			t.Fatalf("expected verified authz sign to populate service context: %+v", value)
		}
		if value.ApiMethod != constant.RequestMethodGrpcString || value.ApiPath != "/acme.test.v1.TestService/Get" {
			t.Fatalf("expected verified method/path to populate service context: %+v", value)
		}
		if value.UserContext == nil || value.UserContext.AppId != "user-app" {
			t.Fatalf("expected grouped user context: %+v", value)
		}
		if len(value.UserContext.PostIds) != 1 || value.UserContext.PostIds[0] != "post-1" {
			t.Fatalf("expected post ids in grouped user context: %+v", value)
		}
		if value.TargetServiceContext == nil || value.TargetServiceContext.AppId != "svc-app" {
			t.Fatalf("expected grouped route context: %+v", value)
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
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
