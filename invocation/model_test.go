package invocation

import "testing"

func TestBuildTarget_UsesDefaultPortAndClusterDomain(t *testing.T) {
	service := &ServiceDNS{
		Service:   "auth",
		Namespace: "default",
	}

	target, err := BuildTarget(service, &DNSConfig{
		DefaultPort: 9090,
	})
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

func TestServiceDNS_EffectivePort_PrefersExplicitPort(t *testing.T) {
	service := ServiceDNS{
		Service:   "auth",
		Namespace: "default",
		Port:      7001,
	}

	port, err := service.EffectivePort(9090)
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

func TestBuildTarget_NilServiceReturnsValidationError(t *testing.T) {
	target, err := BuildTarget(nil, nil)
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
