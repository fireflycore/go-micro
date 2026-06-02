package authz

import (
	"context"
	"errors"
	"testing"

	"github.com/fireflycore/go-micro/constant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestPrepareOutgoingAuthorityMetadata_PreservesUserAuthorityAndOverridesServiceAuthority(t *testing.T) {
	md := metadata.Pairs(
		constant.UserAuthority, "user-token",
		constant.ServiceAuthority, "old-service-token",
		"authorization", "foreign-authorization",
		constant.AuthzSign, "old-jws",
		constant.UserId, "user-1",
		constant.InvokeAppId, "old-invoke",
		constant.TargetAppId, "old-target",
		constant.AppLanguage, "zh-CN",
		constant.XForwardedFor, "10.0.0.1, 10.0.0.2",
		constant.Session, "session-1",
		constant.TraceParent, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00",
		constant.TraceState, "vendor=value",
		constant.Baggage, "tenant=demo",
		"x-custom-metadata", "should-drop",
	)

	prepared, err := PrepareOutgoingAuthorityMetadata(context.Background(), md, fixedServiceAuthorityProvider("new-service-token"))
	if err != nil {
		t.Fatalf("prepare metadata failed: %v", err)
	}

	if got := prepared.Get(constant.UserAuthority); len(got) == 0 || got[0] != "user-token" {
		t.Fatalf("expected user authority to be preserved, got %v", got)
	}
	if got := prepared.Get(constant.ServiceAuthority); len(got) == 0 || got[0] != "new-service-token" {
		t.Fatalf("expected service authority to be overridden, got %v", got)
	}
	if got := prepared.Get("authorization"); len(got) != 0 {
		t.Fatalf("expected authorization to be removed, got %v", got)
	}
	if got := prepared.Get(constant.AuthzSign); len(got) == 0 || got[0] != "old-jws" {
		t.Fatalf("expected authz sign to be preserved for downstream authz reuse, got %v", got)
	}
	if got := prepared.Get(constant.UserId); len(got) != 0 {
		t.Fatalf("expected stale user id to be removed, got %v", got)
	}
	if got := prepared.Get(constant.InvokeAppId); len(got) != 0 {
		t.Fatalf("expected stale invoke app id to be removed, got %v", got)
	}
	if got := prepared.Get(constant.TargetAppId); len(got) != 0 {
		t.Fatalf("expected stale target app id to be removed, got %v", got)
	}
	if got := prepared.Get(constant.AppLanguage); len(got) == 0 || got[0] != "zh-CN" {
		t.Fatalf("expected app language to be preserved, got %v", got)
	}
	if got := prepared.Get(constant.XForwardedFor); len(got) == 0 || got[0] != "10.0.0.1, 10.0.0.2" {
		t.Fatalf("expected x-forwarded-for to be preserved, got %v", got)
	}
	if got := prepared.Get(constant.Session); len(got) != 0 {
		t.Fatalf("expected session to be removed, got %v", got)
	}
	if got := prepared.Get(constant.TraceParent); len(got) == 0 {
		t.Fatalf("expected traceparent to be preserved")
	}
	if got := prepared.Get(constant.TraceState); len(got) == 0 {
		t.Fatalf("expected tracestate to be preserved")
	}
	if got := prepared.Get(constant.Baggage); len(got) == 0 {
		t.Fatalf("expected baggage to be preserved")
	}
	if got := prepared.Get("x-custom-metadata"); len(got) != 0 {
		t.Fatalf("expected custom metadata to be removed, got %v", got)
	}
}

func TestPrepareOutgoingAuthorityMetadata_RemovesInheritedServiceAuthorityWhenProviderMissing(t *testing.T) {
	prepared, err := PrepareOutgoingAuthorityMetadata(
		context.Background(),
		metadata.Pairs(constant.ServiceAuthority, "old-service-token"),
		nil,
	)
	if err != nil {
		t.Fatalf("prepare metadata failed: %v", err)
	}
	if got := prepared.Get(constant.ServiceAuthority); len(got) != 0 {
		t.Fatalf("expected inherited service authority to be removed, got %v", got)
	}
}

func TestNewServiceAuthorityUnaryClientInterceptor_ReturnsProviderError(t *testing.T) {
	expectedErr := errors.New("fetch failed")
	interceptor := NewServiceAuthorityUnaryClientInterceptor(errorServiceAuthorityProvider{err: expectedErr})

	err := interceptor(
		context.Background(),
		"/acme.auth.v1.AuthService/Check",
		struct{}{},
		&struct{}{},
		&grpc.ClientConn{},
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			t.Fatal("invoker should not be called when provider fails")
			return nil
		},
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

type errorServiceAuthorityProvider struct {
	err error
}

func (p errorServiceAuthorityProvider) ServiceAuthority(context.Context) (string, error) {
	return "", p.err
}

type fixedServiceAuthorityProvider string

func (p fixedServiceAuthorityProvider) ServiceAuthority(context.Context) (string, error) {
	return string(p), nil
}
