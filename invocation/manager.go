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
	// DNSManager 用于把业务服务 DNS 描述解析为最终 Target。
	DNSManager *DNSManager
	// DialFunc 用于创建新的 grpc.ClientConn。
	// 若为空，则使用默认拨号实现。
	DialFunc DialFunc
	// DialOptions 表示创建 grpc.ClientConn 时使用的附加选项。
	DialOptions []grpc.DialOption
}

type connectionManagerConfig struct {
	// dnsManager 保存标准 DNS 目标构建器。
	dnsManager *DNSManager
	// dialFunc 保存最终使用的拨号实现。
	dialFunc DialFunc
	// dialOptions 保存创建连接时的固定拨号选项。
	dialOptions []grpc.DialOption
}

// normalize 补齐 ConnectionManagerOptions 的默认值。
func (o ConnectionManagerOptions) normalize() *connectionManagerConfig {
	// 先构造内部配置对象，后续统一在这份对象上补齐默认值。
	config := &connectionManagerConfig{
		dnsManager: o.DNSManager,
		dialFunc:   o.DialFunc,
	}
	// 若未显式提供 DNS 管理器，则使用一份默认配置。
	if config.dnsManager == nil {
		config.dnsManager = NewDNSManager(nil)
	}
	if config.dialFunc == nil {
		config.dialFunc = DefaultDialFunc
	}
	if len(o.DialOptions) == 0 {
		// 默认注入基础凭据与 OTel client handler。
		config.dialOptions = []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		}
		// 默认选项场景到这里即可返回。
		return config
	}
	// 对外部传入的切片做一次复制，避免后续被调用方继续修改。
	config.dialOptions = append(make([]grpc.DialOption, 0, len(o.DialOptions)), o.DialOptions...)
	return config
}

// ConnectionManager 负责缓存基于 DNS 创建出的 grpc.ClientConn。
//
// 它把“业务服务 DNS -> 目标解析 -> 连接缓存”统一收敛在一处，
// 让业务层无需关心：
// - target 拼装；
// - resolver scheme；
// - 拨号选项；
// - 多次调用的连接复用。
type ConnectionManager struct {
	// mu 保护 conns 与 closed，避免并发 Dial/Close 时出现竞态。
	mu sync.RWMutex

	// options 保存连接管理器初始化后的规范化配置。
	config *connectionManagerConfig
	// conns 按最终 gRPC target 缓存可复用连接。
	conns map[string]*grpc.ClientConn
	// closed 标记当前管理器是否已经关闭。
	closed bool
}

// NewConnectionManager 创建连接管理器。
func NewConnectionManager(options ConnectionManagerOptions) (*ConnectionManager, error) {
	// 先统一补齐默认值，再进行依赖校验。
	config := options.normalize()

	if config.dnsManager == nil {
		return nil, ErrDNSManagerIsNil
	}
	if config.dialFunc == nil {
		return nil, ErrDialFnIsNil
	}

	return &ConnectionManager{
		// 保存归一化后的内部配置。
		config: config,
		// 初始化连接缓存 map。
		conns: make(map[string]*grpc.ClientConn),
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

// Dial 根据 DNS 获取或创建对应的 grpc.ClientConn。
//
// 连接缓存键采用最终 gRPC target，而不是 DNS 原始字段，
// 这样可以保证：
// - 逻辑上等价的服务身份只会生成一条连接；
// - 端口覆盖、cluster domain、resolver scheme 的变化都能体现在缓存键上。
func (m *ConnectionManager) Dial(ctx context.Context, dns *DNS) (*grpc.ClientConn, error) {
	// 先通过 DNS 管理器把业务服务描述转换成稳定目标。
	// 通过 DNS 管理器把业务服务配置转成最终目标。
	target, err := m.config.dnsManager.Build(dns)
	if err != nil {
		return nil, err
	}

	// 缓存键统一使用最终 gRPC target，保证语义等价请求命中同一连接。
	key := target.GRPCTarget()
	// 第一阶段走读锁快路径，尽量减少命中缓存时的锁竞争。
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return nil, ErrConnectionManagerClosed
	}
	if conn, ok := m.conns[key]; ok {
		// 命中缓存时直接复用已有连接，避免重复拨号。
		m.mu.RUnlock()
		return conn, nil
	}
	m.mu.RUnlock()

	// 第二阶段升级到写锁，处理真正需要创建连接的场景。
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, ErrConnectionManagerClosed
	}
	if conn, ok := m.conns[key]; ok {
		// 双检一次，避免多个并发协程同时进入慢路径时重复建连。
		m.mu.Unlock()
		return conn, nil
	}
	// 在锁内只读取固定配置，避免把真实拨号过程放在锁内阻塞其它协程。
	dialFunc := m.config.dialFunc
	dialOptions := m.config.dialOptions
	m.mu.Unlock()

	// 在锁外执行拨号，降低并发场景的串行等待时间。
	conn, err := dialFunc(ctx, *target, dialOptions)
	if err != nil {
		return nil, err
	}

	// 拨号完成后重新加锁，把连接安全写入缓存。
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		// 若管理器在拨号期间已被关闭，则主动关闭新建连接。
		_ = conn.Close()
		return nil, ErrConnectionManagerClosed
	}
	if cached, ok := m.conns[key]; ok {
		// 若别的协程已经抢先写入缓存，则关闭当前新连接并复用已有连接。
		_ = conn.Close()
		return cached, nil
	}
	// 仅在拨号成功后写入缓存，避免缓存无效连接。
	m.conns[key] = conn
	// 返回缓存中的新连接。
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
