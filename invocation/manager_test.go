package invocation

import (
	"context"
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

	service := &ServiceDNS{
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

	_, err = manager.Dial(context.Background(), &ServiceDNS{
		Service:   "auth",
		Namespace: "default",
	})
	if err != ErrConnectionManagerClosed {
		t.Fatalf("expected ErrConnectionManagerClosed, got %v", err)
	}
}
