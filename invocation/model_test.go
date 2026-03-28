package invocation

import "testing"

func TestBuildTarget_UsesDefaultPortAndClusterDomain(t *testing.T) {
	ref := ServiceRef{
		Service:   "auth",
		Namespace: "default",
		Env:       "dev",
	}

	target, err := BuildTarget(ref, TargetOptions{
		DefaultPort: 9000,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if target.Host != "auth.default.svc.cluster.local" {
		t.Fatalf("unexpected host: %s", target.Host)
	}
	if target.Port != 9000 {
		t.Fatalf("unexpected port: %d", target.Port)
	}
	if target.GRPCTarget() != "dns:///auth.default.svc.cluster.local:9000" {
		t.Fatalf("unexpected grpc target: %s", target.GRPCTarget())
	}
}

func TestServiceRef_EffectivePort_PrefersExplicitPort(t *testing.T) {
	ref := ServiceRef{
		Service:   "auth",
		Namespace: "default",
		Port:      7001,
	}

	port, err := ref.EffectivePort(9000)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if port != 7001 {
		t.Fatalf("expected explicit port 7001, got %d", port)
	}
}
