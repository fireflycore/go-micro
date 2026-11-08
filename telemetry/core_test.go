package telemetry

import (
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

func TestNewResourceIncludesEnvironment(t *testing.T) {
	res, err := newResource(&Resource{
		ServiceId:         "app-id",
		Environment:       "prod",
		ServiceName:       "app",
		ServiceVersion:    "v0.1.0",
		ServiceNamespace:  "lhdht",
		ServiceInstanceId: "instance-id",
	})
	if err != nil {
		t.Fatalf("new resource failed: %v", err)
	}
	if got, want := res.SchemaURL(), resource.Default().SchemaURL(); got != want {
		t.Fatalf("resource schema = %q, want %q", got, want)
	}
	for key, want := range map[string]string{
		"service.name": "app", "service.version": "v0.1.0",
		"service.namespace": "lhdht", "service.instance.id": "instance-id",
	} {
		if got, ok := res.Set().Value(attribute.Key(key)); !ok || got.AsString() != want {
			t.Fatalf("resource %s = %q, want %q", key, got.AsString(), want)
		}
	}
	if got, ok := res.Set().Value(attribute.Key("deployment.environment")); !ok || got.AsString() != "prod" {
		t.Fatalf("unexpected deployment.environment: got=%s want=%s", got.AsString(), "prod")
	}
}
