package invocation

import "testing"

func TestTarget_GRPCTarget_UsesResolverScheme(t *testing.T) {
	target := Target{
		ResolverScheme: "dns",
		Host:           "auth.default.svc.cluster.local",
		Port:           9090,
	}

	if got := target.GRPCTarget(); got != "dns:///auth.default.svc.cluster.local:9090" {
		t.Fatalf("unexpected grpc target: %s", got)
	}
}

func TestTarget_GRPCTarget_WithoutResolverSchemeFallsBackToAddress(t *testing.T) {
	target := Target{
		Host: "auth.default.svc.cluster.local",
		Port: 9090,
	}

	if got := target.GRPCTarget(); got != "auth.default.svc.cluster.local:9090" {
		t.Fatalf("unexpected grpc target fallback: %s", got)
	}
}

func TestTarget_Validate_RejectsEmptyHost(t *testing.T) {
	target := Target{Port: 9090}

	if err := target.Validate(); err != ErrTargetHostEmpty {
		t.Fatalf("expected %v, got %v", ErrTargetHostEmpty, err)
	}
}
