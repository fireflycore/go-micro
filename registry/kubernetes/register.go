package kubernetes

import (
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// Register 基于 K8S/Istio 的服务注册实现。
// 职责：
// 1. 托管 gRPC Health Check (Liveness/Readiness Probe)。
// 2. 开启 gRPC Reflection (便于 K8S 内部调试)。
// 3. 提供生产级的优雅停机 (Graceful Shutdown) 流程与生命周期钩子。
type Register struct {
	server       *health.Server
	grpcServer   *grpc.Server
	log          *zap.Logger
	shutdownWait time.Duration // 收到信号后，关闭 Server 前的等待时间

	// 生命周期钩子
	mu           sync.Mutex
	shutdownHook []func()
}

// NewRegister 创建一个基于 Health Check 的注册器。
// 会自动开启 Health Check 和 Server Reflection。
// 支持通过环境变量 K8S_SHUTDOWN_WAIT 配置等待时间（默认 10s）。
func NewRegister(s *grpc.Server) *Register {
	// 1. 注册 Health Server
	hs := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, hs)

	// 2. 注册 Reflection
	reflection.Register(s)

	// 3. 读取配置
	wait := 10 * time.Second
	if v := os.Getenv("K8S_SHUTDOWN_WAIT"); v != "" {
		if s, err := strconv.Atoi(v); err == nil && s > 0 {
			wait = time.Duration(s) * time.Second
		}
	}

	return &Register{
		server:       hs,
		grpcServer:   s,
		shutdownWait: wait,
		shutdownHook: make([]func(), 0),
	}
}

// Start 标记服务状态为 SERVING。
func (r *Register) Start() error {
	r.server.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	if r.log != nil {
		r.log.Info("k8s registry started",
			zap.String("status", "SERVING"),
			zap.Bool("reflection", true),
			zap.Duration("shutdown_wait", r.shutdownWait),
		)
	}
	return nil
}

// Stop 标记服务状态为 NOT_SERVING。
func (r *Register) Stop() {
	r.server.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	if r.log != nil {
		r.log.Info("k8s registry stopped: health check not serving")
	}
}

// SetStatus 手动设置特定服务的健康状态。
// 用于业务逻辑中主动报告部分组件不可用（如 DB 断开）。
// service: 服务名，空字符串代表整个 Server。
func (r *Register) SetStatus(service string, status grpc_health_v1.HealthCheckResponse_ServingStatus) {
	r.server.SetServingStatus(service, status)
	if r.log != nil {
		r.log.Info("health status changed", zap.String("service", service), zap.String("status", status.String()))
	}
}

// WithLog 设置日志记录器。
func (r *Register) WithLog(l *zap.Logger) {
	r.log = l
}

// OnShutdown 注册在优雅停机过程中执行的钩子函数。
// 这些函数会在 Health Check 停止后、gRPC Server 关闭前执行（并行执行或按需处理，此处为顺序执行）。
// 典型用途：关闭数据库连接、清理 Redis 客户端、停止后台消费者等。
func (r *Register) OnShutdown(fn func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shutdownHook = append(r.shutdownHook, fn)
}

// RunBlock 启动一个阻塞的生命周期管理器。
// 流程：Signal -> Health=NotServing -> Wait(Drain) -> Hooks -> GracefulStop
func (r *Register) RunBlock() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	if r.log != nil {
		r.log.Info("server is running, waiting for signals...")
	}

	sig := <-quit
	if r.log != nil {
		r.log.Info("received signal, starting graceful shutdown", zap.String("signal", sig.String()))
	}

	// 1. 摘流：标记 Health 为 NOT_SERVING
	r.Stop()

	// 2. 缓冲：等待 K8S 网络传播
	if r.shutdownWait > 0 {
		if r.log != nil {
			r.log.Info("waiting for traffic drain", zap.Duration("duration", r.shutdownWait))
		}
		time.Sleep(r.shutdownWait)
	}

	// 3. 资源清理：执行注册的 Shutdown Hooks
	// 在 gRPC Stop 之前执行，确保像 DB 这样的资源在请求处理完之前（GracefulStop 期间）还可用？
	// 不，GracefulStop 会等待所有 RPC 完成。如果 RPC 依赖 DB，那么 DB 必须在 GracefulStop *之后* 关闭。
	// 但也有一些资源（如消息队列消费者）需要在 GracefulStop *之前* 停止，不再产生新任务。
	// 为了通用性，我们采用“后进先出”或“顺序执行”策略，但通常资源关闭建议放在 GracefulStop 之后，
	// 或者分为 BeforeStop 和 AfterStop。
	// 这里简化为：先 GracefulStop（保证没有新请求，旧请求处理完），再执行 Hooks（关闭 DB 等底层资源）。

	if r.log != nil {
		r.log.Info("executing grpc graceful stop")
	}
	r.grpcServer.GracefulStop()

	// 4. 执行自定义清理逻辑 (如关闭 DB)
	r.executeHooks()

	if r.log != nil {
		r.log.Info("server exited gracefully")
	}
}

func (r *Register) executeHooks() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.shutdownHook) == 0 {
		return
	}

	if r.log != nil {
		r.log.Info("executing shutdown hooks", zap.Int("count", len(r.shutdownHook)))
	}

	// 倒序执行，模拟 defer 行为
	for i := len(r.shutdownHook) - 1; i >= 0; i-- {
		r.shutdownHook[i]()
	}
}
