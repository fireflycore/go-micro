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

func TestTarget_Validate_RejectsZeroPort(t *testing.T) {
	target := Target{Host: "auth.default.svc.cluster.local"}

	if err := target.Validate(); err != ErrTargetPortInvalid {
		t.Fatalf("expected %v, got %v", ErrTargetPortInvalid, err)
	}
}

func TestTarget_Address_UsesCachedValueWhenAvailable(t *testing.T) {
	target := Target{
		Host: "auth.default.svc.cluster.local",
		Port: 9090,
	}
	target.cacheDerivedStrings()

	if got := target.Address(); got != "auth.default.svc.cluster.local:9090" {
		t.Fatalf("unexpected address: %s", got)
	}
	if target.address == "" {
		t.Fatal("expected cached address to be populated")
	}
}

func TestTarget_CacheDerivedStrings_WithoutResolverSchemeUsesAddress(t *testing.T) {
	target := &Target{
		Host: "auth.default.svc.cluster.local",
		Port: 9090,
	}

	target.cacheDerivedStrings()

	if target.grpcTarget != "auth.default.svc.cluster.local:9090" {
		t.Fatalf("unexpected cached grpc target: %s", target.grpcTarget)
	}
}

func TestTarget_CacheDerivedStrings_AllowsNilReceiver(t *testing.T) {
	var target *Target
	target.cacheDerivedStrings()
}
