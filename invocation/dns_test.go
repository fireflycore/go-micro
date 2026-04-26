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

func TestEffectivePort_UsesDefaultPortWhenServicePortMissing(t *testing.T) {
	port, err := effectivePort(&srv.DNS{}, 9090)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if port != 9090 {
		t.Fatalf("expected default port 9090, got %d", port)
	}
}

func TestEffectivePort_ReturnsErrorWhenNoPortAvailable(t *testing.T) {
	port, err := effectivePort(nil, 0)
	if err != ErrTargetPortInvalid {
		t.Fatalf("expected %v, got %v", ErrTargetPortInvalid, err)
	}
	if port != 0 {
		t.Fatalf("expected zero port, got %d", port)
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

func TestDNSManager_Config_NilReceiverUsesDefaults(t *testing.T) {
	var manager *DNSManager
	config := manager.Config()

	if config.DefaultNamespace != "default" {
		t.Fatalf("unexpected default namespace: %s", config.DefaultNamespace)
	}
	if config.ResolverScheme != DefaultResolverScheme {
		t.Fatalf("unexpected resolver scheme: %s", config.ResolverScheme)
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

func TestValidateDNS_RejectsMissingNamespace(t *testing.T) {
	err := validateDNS(&srv.DNS{Service: "auth"})
	if err != ErrNamespaceEmpty {
		t.Fatalf("expected %v, got %v", ErrNamespaceEmpty, err)
	}
}

func TestValidateDNS_RejectsNilDNS(t *testing.T) {
	err := validateDNS(nil)
	if err != ErrServiceNameEmpty {
		t.Fatalf("expected %v, got %v", ErrServiceNameEmpty, err)
	}
}
