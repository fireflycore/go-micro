package invocation

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fireflycore/go-micro/constant"
	svc "github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var (
	// benchmarkConnSink 防止编译器把连接相关结果优化掉。
	benchmarkConnSink *grpc.ClientConn
	// benchmarkTargetSink 防止目标字符串被编译器消除。
	benchmarkTargetSink string
	// benchmarkErrSink 防止错误返回被编译器消除。
	benchmarkErrSink error
)

// BenchmarkDNSManagerBuild 对比 DNSManager 构建目标的前后实现。
func BenchmarkDNSManagerBuild(b *testing.B) {
	// 使用固定配置保证 baseline 和 optimized 输入一致。
	manager := NewDNSManager(&DNSConfig{
		DefaultNamespace: "default",
		DefaultPort:      9090,
	})

	b.Run("baseline_old", func(b *testing.B) {
		// 报告分配次数，便于观察复制和格式化开销。
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// 每轮构造一份相同输入，避免输入本身被污染。
			dns := svc.DNS{Service: "auth"}
			// 调用旧路径 helper，模拟优化前逻辑。
			target, err := oldBuildDNSManagerTarget(manager, &dns)
			benchmarkErrSink = err
			if target != nil {
				// 消费结果，避免编译器移除整段逻辑。
				benchmarkTargetSink = target.GRPCTarget()
			}
		}
	})

	b.Run("optimized", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// 同样保持每轮输入一致。
			dns := svc.DNS{Service: "auth"}
			// 调用当前优化后的正式实现。
			target, err := manager.Build(&dns)
			benchmarkErrSink = err
			if target != nil {
				benchmarkTargetSink = target.GRPCTarget()
			}
		}
	})
}

func BenchmarkConnectionManagerDialCachedParallel(b *testing.B) {
	// 固定一个可命中缓存的服务标识。
	dns := &svc.DNS{Service: "auth", Namespace: "default"}
	// 所有并发 worker 共享同一个上下文即可。
	ctx := context.Background()

	b.Run("baseline_old", func(b *testing.B) {
		// 构造旧版连接管理器。
		manager := newOldConnectionManager()
		// 先预热一次连接缓存，保证 benchmark 关注缓存命中路径。
		conn, err := manager.Dial(ctx, dns)
		if err != nil {
			b.Fatalf("warm cache failed: %v", err)
		}
		benchmarkConnSink = conn

		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// 并发命中旧版缓存路径。
				conn, err := manager.Dial(ctx, dns)
				benchmarkErrSink = err
				benchmarkConnSink = conn
			}
		})
	})

	b.Run("optimized", func(b *testing.B) {
		// 构造当前优化后的连接管理器。
		manager, err := NewConnectionManager(ConnectionManagerOptions{
			DNSManager: NewDNSManager(&DNSConfig{DefaultPort: 9090}),
			DialFunc: func(ctx context.Context, target Target, options []grpc.DialOption) (*grpc.ClientConn, error) {
				// 这里返回空连接对象即可，benchmark 关注的是管理器内部开销。
				return &grpc.ClientConn{}, nil
			},
		})
		if err != nil {
			b.Fatalf("new manager failed: %v", err)
		}
		conn, err := manager.Dial(ctx, dns)
		if err != nil {
			b.Fatalf("warm cache failed: %v", err)
		}
		benchmarkConnSink = conn

		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// 并发命中优化后的缓存路径。
				conn, err := manager.Dial(ctx, dns)
				benchmarkErrSink = err
				benchmarkConnSink = conn
			}
		})
	})
}

func BenchmarkUnaryInvokerInvoke(b *testing.B) {
	// 构造带入站 metadata 的上下文，模拟真实链路透传场景。
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.UserId, "u-1",
		"x-request-id", "req-1",
	))
	// 固定一个被调用服务。
	dns := &svc.DNS{Service: "auth", Namespace: "default"}
	// 构造统一调用器，并用假的 Dialer/InvokeFunc 避免真实网络开销干扰。
	invoker := &UnaryInvoker{
		Dialer:            testDialer{conn: &grpc.ClientConn{}},
		ServiceAppId:      "config",
		ServiceInstanceId: "config-1",
		Timeout:           3 * time.Second,
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			// 消费连接对象，防止调用被优化掉。
			benchmarkConnSink = conn
			return nil
		},
	}

	b.Run("baseline_old", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// 调用旧版 unary 主流程。
			benchmarkErrSink = oldUnaryInvoke(invoker, ctx, dns, "/acme.auth.v1.AuthService/Check", &struct{}{}, &struct{}{})
		}
	})

	b.Run("optimized", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// 调用当前优化后的 unary 主流程。
			benchmarkErrSink = invoker.Invoke(ctx, dns, "/acme.auth.v1.AuthService/Check", &struct{}{}, &struct{}{})
		}
	})
}

// BenchmarkRemoteServiceManagedInvoke 对比多服务管理器的直接调用路径。
func BenchmarkRemoteServiceManagedInvoke(b *testing.B) {
	// 预注册两组业务服务，模拟真实多服务装配场景。
	services := NewRemoteServiceManaged(
		&UnaryInvoker{
			Dialer: testDialer{conn: &grpc.ClientConn{}},
			InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
				// 消费连接对象，防止编译器消除。
				benchmarkConnSink = conn
				return nil
			},
		},
		svc.DNS{Service: "auth", Namespace: "default"},
		svc.DNS{Service: "app", Namespace: "default"},
	)

	b.Run("baseline_old", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// 旧路径会先取 DNS，再构造 caller，再发起调用。
			benchmarkErrSink = oldRemoteServiceManagedInvoke(services, context.Background(), "auth", "/acme.auth.v1.AuthService/Check", &struct{}{}, &struct{}{})
		}
	})

	b.Run("optimized", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// 新路径直接查表并复用 invoker。
			benchmarkErrSink = services.Invoke(context.Background(), "auth", "/acme.auth.v1.AuthService/Check", &struct{}{}, &struct{}{})
		}
	})
}

// oldBuildDNSManagerTarget 模拟优化前 DNSManager.Build 的关键执行路径。
func oldBuildDNSManagerTarget(manager *DNSManager, dns *svc.DNS) (*Target, error) {
	if dns == nil {
		// 旧实现同样会在 nil 输入时构造一份空对象。
		dns = &svc.DNS{}
	}
	// 旧路径第一次读取配置副本。
	config := manager.Config()
	if strings.TrimSpace(dns.Namespace) == "" {
		dns.Namespace = config.DefaultNamespace
	}
	if strings.TrimSpace(dns.ServiceType) == "" {
		dns.ServiceType = config.DefaultServiceType
	}
	if strings.TrimSpace(dns.ClusterDomain) == "" {
		dns.ClusterDomain = config.DefaultClusterDomain
	}
	if dns.Port == 0 {
		dns.Port = config.DefaultPort
	}
	if err := validateDNS(dns); err != nil {
		return &Target{}, err
	}
	// 旧路径再次读取配置副本来拿默认端口。
	port, err := effectivePort(dns, manager.Config().DefaultPort)
	if err != nil {
		return &Target{}, err
	}
	target := Target{
		// 旧路径第三次读取配置副本来拿 resolver scheme。
		ResolverScheme: manager.Config().ResolverScheme,
		Host: fmt.Sprintf(
			"%s.%s.%s.%s",
			strings.TrimSpace(dns.Service),
			strings.TrimSpace(dns.Namespace),
			strings.TrimSpace(dns.ServiceType),
			strings.TrimSpace(dns.ClusterDomain),
		),
		Port: port,
	}
	if err := target.Validate(); err != nil {
		return &Target{}, err
	}
	// 返回未预缓存派生字段的旧目标对象。
	return &target, nil
}

// oldConnectionManager 模拟优化前“整段 Dial 持有互斥锁”的实现。
type oldConnectionManager struct {
	mu      sync.Mutex
	options ConnectionManagerOptions
	conns   map[string]*grpc.ClientConn
	closed  bool
}

func newOldConnectionManager() *oldConnectionManager {
	return &oldConnectionManager{
		options: ConnectionManagerOptions{
			DNSManager: NewDNSManager(&DNSConfig{DefaultPort: 9090}),
			DialFunc: func(ctx context.Context, target Target, options []grpc.DialOption) (*grpc.ClientConn, error) {
				// 基准里不做真实网络拨号。
				return &grpc.ClientConn{}, nil
			},
		},
		conns: make(map[string]*grpc.ClientConn),
	}
}

func (m *oldConnectionManager) Dial(ctx context.Context, dns *svc.DNS) (*grpc.ClientConn, error) {
	// 旧实现从进入 Dial 到返回全程持有互斥锁。
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, ErrConnectionManagerClosed
	}
	// 在锁内完成目标构建。
	target, err := m.options.DNSManager.Build(dns)
	if err != nil {
		return nil, err
	}
	key := target.GRPCTarget()
	if conn, ok := m.conns[key]; ok {
		// 命中缓存直接返回。
		return conn, nil
	}
	// 未命中时也在锁内执行拨号。
	conn, err := m.options.DialFunc(ctx, *target, m.options.DialOptions)
	if err != nil {
		return nil, err
	}
	// 成功后写入缓存。
	m.conns[key] = conn
	return conn, nil
}

// oldUnaryInvoke 模拟优化前 UnaryInvoker.Invoke 的关键路径。
func oldUnaryInvoke(u *UnaryInvoker, ctx context.Context, dns *svc.DNS, method string, req any, resp any, callOptions ...grpc.CallOption) error {
	if u == nil || u.Dialer == nil {
		return ErrInvokerDialerIsNil
	}
	if method == "" {
		return ErrInvokeMethodEmpty
	}

	resolvedMetadata := resolveOutgoingMetadata(ctx, u.ServiceAppId, u.ServiceInstanceId)
	conn, err := u.Dialer.Dial(ctx, dns)
	if err != nil {
		return err
	}
	// 旧路径通过公共函数构造出站上下文。
	outCtx, cancel := NewOutgoingCallContext(ctx, resolvedMetadata, u.Timeout)
	defer cancel()

	invokeFunc := u.InvokeFunc
	if invokeFunc == nil {
		// 旧路径在热路径上临时创建匿名函数。
		invokeFunc = func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			return conn.Invoke(ctx, method, req, resp, options...)
		}
	}
	// 使用最终的 invoke 函数发起调用。
	return invokeFunc(outCtx, conn, method, req, resp, callOptions...)
}

// oldRemoteServiceManagedInvoke 模拟优化前先创建 caller 再发起调用的路径。
func oldRemoteServiceManagedInvoke(r *RemoteServiceManaged, ctx context.Context, serviceName string, method string, req any, resp any, callOptions ...grpc.CallOption) error {
	// 对外先拿一份 DNS 副本。
	dns, err := r.DNS(serviceName)
	if err != nil {
		return err
	}
	// 再额外创建一个 RemoteServiceCaller 包装层。
	caller := NewRemoteServiceCaller(r.invoker, dns)
	// 最终通过 caller 间接发起调用。
	return caller.Invoke(ctx, method, req, resp, callOptions...)
}

// BenchmarkTargetGRPCTarget 对比目标字符串格式化前后的差异。
func BenchmarkTargetGRPCTarget(b *testing.B) {
	target := &Target{
		ResolverScheme: "dns",
		Host:           "auth.default.svc.cluster.local",
		Port:           9090,
	}
	target.cacheDerivedStrings()

	b.Run("baseline_old", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// 旧路径每次都重新拼接 address 和 grpc target。
			address := net.JoinHostPort(strings.TrimSpace(target.Host), fmt.Sprint(target.Port))
			benchmarkTargetSink = fmt.Sprintf("%s:///%s", target.ResolverScheme, address)
		}
	})

	b.Run("optimized", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// 新路径直接读取缓存好的字符串。
			benchmarkTargetSink = target.GRPCTarget()
		}
	})
}
