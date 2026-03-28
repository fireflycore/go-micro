package invocation

import (
	"context"
	"sync"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DialFunc 表示底层拨号函数。
//
// 把拨号过程抽象成函数有两个目的：
// 1. 让 ConnectionManager 在不依赖具体后端的情况下复用连接；
// 2. 让单元测试可以替换真实拨号逻辑，避免触发真实网络连接。
type DialFunc func(ctx context.Context, target Target, options []grpc.DialOption) (*grpc.ClientConn, error)

// ConnectionManagerOptions 定义连接管理器的配置。
type ConnectionManagerOptions struct {
	// Locator 用于把 ServiceRef 解析为最终 Target。
	Locator Locator
	// DialFunc 用于创建新的 grpc.ClientConn。
	// 若为空，则使用默认拨号实现。
	DialFunc DialFunc
	// DialOptions 表示创建 grpc.ClientConn 时使用的附加选项。
	DialOptions []grpc.DialOption
}

// normalize 补齐 ConnectionManagerOptions 的默认值。
func (o ConnectionManagerOptions) normalize() ConnectionManagerOptions {
	if o.DialFunc == nil {
		o.DialFunc = DefaultDialFunc
	}
	if len(o.DialOptions) == 0 {
		o.DialOptions = []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		}
	}
	return o
}

// ConnectionManager 负责缓存基于 ServiceRef 创建出的 grpc.ClientConn。
//
// 它把“服务身份 -> 目标解析 -> 连接缓存”统一收敛在一处，
// 让业务层无需关心：
// - target 拼装；
// - resolver scheme；
// - 拨号选项；
// - 多次调用的连接复用。
type ConnectionManager struct {
	mu sync.Mutex

	options ConnectionManagerOptions
	conns   map[string]*grpc.ClientConn
	closed  bool
}

// NewConnectionManager 创建连接管理器。
func NewConnectionManager(options ConnectionManagerOptions) (*ConnectionManager, error) {
	options = options.normalize()

	if options.Locator == nil {
		return nil, ErrLocatorIsNil
	}
	if options.DialFunc == nil {
		return nil, ErrDialFnIsNil
	}

	return &ConnectionManager{
		options: options,
		conns:   make(map[string]*grpc.ClientConn),
	}, nil
}

// DefaultDialFunc 是默认的 grpc.ClientConn 创建逻辑。
//
// 当前实现采用 grpc.NewClient，并默认启用：
// - insecure credentials：便于在内部受控网络中快速起步；
// - otelgrpc client handler：保证调用链路自动接入 OTel。
//
// 后续若需要在具体实现中启用 mTLS、自定义 resolver 或更多 dial option，
// 可以通过 ConnectionManagerOptions 覆盖该行为。
func DefaultDialFunc(_ context.Context, target Target, options []grpc.DialOption) (*grpc.ClientConn, error) {
	return grpc.NewClient(target.GRPCTarget(), options...)
}

// Dial 根据 ServiceRef 获取或创建对应的 grpc.ClientConn。
//
// 连接缓存键采用最终 gRPC target，而不是 ServiceRef 原始字段，
// 这样可以保证：
// - 逻辑上等价的服务身份只会生成一条连接；
// - 端口覆盖、cluster domain、resolver scheme 的变化都能体现在缓存键上。
func (m *ConnectionManager) Dial(ctx context.Context, ref ServiceRef) (*grpc.ClientConn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, ErrConnectionManagerClosed
	}

	target, err := m.options.Locator.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}

	key := target.GRPCTarget()
	if conn, ok := m.conns[key]; ok {
		return conn, nil
	}

	conn, err := m.options.DialFunc(ctx, target, m.options.DialOptions)
	if err != nil {
		return nil, err
	}

	m.conns[key] = conn
	return conn, nil
}

// Close 关闭连接管理器及其持有的全部 grpc.ClientConn。
//
// Close 会尽最大努力关闭所有连接；
// 若中途出现错误，当前实现返回第一条错误并继续关闭剩余连接。
func (m *ConnectionManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true

	var firstErr error
	for key, conn := range m.conns {
		if conn == nil {
			delete(m.conns, key)
			continue
		}
		if err := conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(m.conns, key)
	}

	return firstErr
}
