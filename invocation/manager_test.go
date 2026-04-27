package invocation

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestConnectionManager_Dial_CachesByResolvedTarget(t *testing.T) {
	var dialCount atomic.Int32

	manager, err := NewConnectionManager(ConnectionManagerOptions{
		DNSManager: NewDNSManager(&DNSConfig{DefaultPort: 9090}),
		DialFunc: func(ctx context.Context, target Target, options []grpc.DialOption) (*grpc.ClientConn, error) {
			dialCount.Add(1)
			return grpc.NewClient("passthrough:///auth.default.svc.cluster.local:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	defer func() { _ = manager.Close() }()

	service := &DNS{
		Service:   "auth",
		Namespace: "default",
	}

	conn1, err := manager.Dial(context.Background(), service)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	conn2, err := manager.Dial(context.Background(), service)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if conn1 != conn2 {
		t.Fatalf("expected cached connection reuse")
	}
	if dialCount.Load() != 1 {
		t.Fatalf("expected dial count 1, got %d", dialCount.Load())
	}
}

func TestConnectionManager_Dial_AfterCloseReturnsError(t *testing.T) {
	manager, err := NewConnectionManager(ConnectionManagerOptions{
		DNSManager: NewDNSManager(&DNSConfig{DefaultPort: 9090}),
		DialFunc: func(ctx context.Context, target Target, options []grpc.DialOption) (*grpc.ClientConn, error) {
			return grpc.NewClient("passthrough:///auth.default.svc.cluster.local:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if err := manager.Close(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	_, err = manager.Dial(context.Background(), &DNS{
		Service:   "auth",
		Namespace: "default",
	})
	if err != ErrConnectionManagerClosed {
		t.Fatalf("expected ErrConnectionManagerClosed, got %v", err)
	}
}

func TestNewConnectionManager_UsesDefaultsWhenOptionsEmpty(t *testing.T) {
	manager, err := NewConnectionManager(ConnectionManagerOptions{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if manager.config == nil {
		t.Fatal("expected normalized config")
	}
	if manager.config.dnsManager == nil {
		t.Fatal("expected default dns manager")
	}
	if manager.config.dialFunc == nil {
		t.Fatal("expected default dial func")
	}
	if len(manager.config.dialOptions) == 0 {
		t.Fatal("expected default dial options")
	}
}

func TestConnectionManager_Close_IsIdempotent(t *testing.T) {
	manager, err := NewConnectionManager(ConnectionManagerOptions{
		DNSManager: NewDNSManager(nil),
		DialFunc: func(ctx context.Context, target Target, options []grpc.DialOption) (*grpc.ClientConn, error) {
			return &grpc.ClientConn{}, nil
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if err := manager.Close(); err != nil {
		t.Fatalf("expected nil error on first close, got %v", err)
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("expected nil error on second close, got %v", err)
	}
}

func TestConnectionManager_Dial_PropagatesDialError(t *testing.T) {
	expectedErr := errors.New("dial failed")
	manager, err := NewConnectionManager(ConnectionManagerOptions{
		DNSManager: NewDNSManager(nil),
		DialFunc: func(ctx context.Context, target Target, options []grpc.DialOption) (*grpc.ClientConn, error) {
			return nil, expectedErr
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	_, err = manager.Dial(context.Background(), &DNS{Service: "auth", Namespace: "default"})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestDefaultDialFunc_RejectsInvalidTarget(t *testing.T) {
	_, err := DefaultDialFunc(context.Background(), Target{}, nil)
	if err == nil {
		t.Fatal("expected error for invalid target")
	}
}
