package invocation

import (
	"context"
	"testing"

	"github.com/fireflycore/go-micro/constant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestRemoteServiceManaged_Caller_ReturnsConfiguredService(t *testing.T) {
	services := NewRemoteServiceManaged(
		&UnaryInvoker{Dialer: testDialer{conn: &grpc.ClientConn{}}},
		DNS{Service: "auth", Namespace: "default"},
		DNS{Service: "app", Namespace: "default"},
	)

	caller, err := services.Caller("auth")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if caller.Service == nil || caller.Service.Service != "auth" {
		t.Fatalf("unexpected caller service: %+v", caller.Service)
	}
}

func TestRemoteServiceManaged_Invoke_UsesSharedInvokerAndInjectsServiceIdentity(t *testing.T) {
	invoker := &UnaryInvoker{
		Dialer:            testDialer{conn: &grpc.ClientConn{}},
		ServiceAppId:      "config",
		ServiceInstanceId: "config-1",
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				t.Fatal("expected outgoing metadata")
			}
			if got := md.Get(constant.ServiceAppId); len(got) == 0 || got[0] != "config" {
				t.Fatalf("unexpected service app id metadata: %v", got)
			}
			if got := md.Get(constant.ServiceInstanceId); len(got) == 0 || got[0] != "config-1" {
				t.Fatalf("unexpected service instance id metadata: %v", got)
			}
			return nil
		},
	}

	services := NewRemoteServiceManaged(
		invoker,
		DNS{Service: "auth", Namespace: "default"},
		DNS{Service: "app", Namespace: "default"},
	)

	err := services.Invoke(
		context.Background(),
		"auth",
		"/acme.auth.app.v1.AuthAppService/GetAppSecret",
		&struct{}{},
		&struct{}{},
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestRemoteServiceManaged_Caller_ReturnsNotFoundForUnknownService(t *testing.T) {
	services := NewRemoteServiceManaged(nil, DNS{Service: "auth", Namespace: "default"})

	_, err := services.Caller("billing")
	if err != ErrRemoteServiceNotFound {
		t.Fatalf("expected %v, got %v", ErrRemoteServiceNotFound, err)
	}
}

func TestRemoteServiceManaged_DNS_ReturnsClone(t *testing.T) {
	services := NewRemoteServiceManaged(nil, DNS{Service: "auth", Namespace: "default"})

	dns, err := services.DNS("auth")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	dns.Namespace = "changed"

	again, err := services.DNS("auth")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if again.Namespace != "default" {
		t.Fatalf("expected stored service to stay unchanged, got %s", again.Namespace)
	}
}

func TestRemoteServiceManaged_Invoke_ReturnsErrorWhenInvokerMissing(t *testing.T) {
	services := NewRemoteServiceManaged(nil, DNS{Service: "auth", Namespace: "default"})

	err := services.Invoke(context.Background(), "auth", "/acme.auth.v1.AuthService/Check", &struct{}{}, &struct{}{})
	if err != ErrInvokerDialerIsNil {
		t.Fatalf("expected %v, got %v", ErrInvokerDialerIsNil, err)
	}
}

func TestRemoteServiceManaged_New_SkipsEmptyServiceName(t *testing.T) {
	services := NewRemoteServiceManaged(nil,
		DNS{Service: "auth", Namespace: "default"},
		DNS{Service: " ", Namespace: "default"},
	)

	if _, err := services.DNS("auth"); err != nil {
		t.Fatalf("expected auth service to exist, got %v", err)
	}
	if _, err := services.DNS(" "); err != ErrRemoteServiceNotFound {
		t.Fatalf("expected %v, got %v", ErrRemoteServiceNotFound, err)
	}
}

func TestRemoteServiceManaged_DNS_ReturnsNotFoundForNilReceiver(t *testing.T) {
	var services *RemoteServiceManaged

	_, err := services.DNS("auth")
	if err != ErrRemoteServiceNotFound {
		t.Fatalf("expected %v, got %v", ErrRemoteServiceNotFound, err)
	}
}
