package invocation

import (
	"testing"

	srv "github.com/fireflycore/go-micro/service"
)

func TestDNSManager_Build_UsesDefaultPortAndClusterDomain(t *testing.T) {
	service := &srv.DNS{
		Service:   "auth",
		Namespace: "default",
	}

	target, err := NewDNSManager(&DNSConfig{
		DefaultPort: 9090,
	}).Build(service)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if target.Host != "auth.default.svc.cluster.local" {
		t.Fatalf("unexpected host: %s", target.Host)
	}
	if target.Port != 9090 {
		t.Fatalf("unexpected port: %d", target.Port)
	}
	if target.GRPCTarget() != "dns:///auth.default.svc.cluster.local:9090" {
		t.Fatalf("unexpected grpc target: %s", target.GRPCTarget())
	}
}

func TestEffectivePort_PrefersExplicitPort(t *testing.T) {
	port, err := effectivePort(&srv.DNS{Port: 7001}, 9090)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if port != 7001 {
		t.Fatalf("expected explicit port 7001, got %d", port)
	}
}

func TestNewDNSManager_NilConfigUsesDefaults(t *testing.T) {
	manager := NewDNSManager(nil)
	config := manager.Config()

	if config.DefaultNamespace != "default" {
		t.Fatalf("unexpected default namespace: %s", config.DefaultNamespace)
	}
	if config.DefaultPort != DefaultServicePort {
		t.Fatalf("unexpected default port: %d", config.DefaultPort)
	}
}

func TestDNSManager_Build_NilServiceReturnsValidationError(t *testing.T) {
	target, err := NewDNSManager(nil).Build(nil)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if target == nil {
		t.Fatal("expected non-nil target pointer for error case")
	}
	if target.Host != "" || target.Port != 0 {
		t.Fatalf("unexpected target on error: %+v", target)
	}
}
