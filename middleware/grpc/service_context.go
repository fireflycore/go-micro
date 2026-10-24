package gm

import (
	"context"
	"net"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// ServiceContextInterceptorOptions 配置实际服务事实与入口信任边界。
type ServiceContextInterceptorOptions struct {
	// ServiceAppId 来自本地配置，不能接受客户端同名头覆盖。
	ServiceAppId string
	// ServiceInstanceId 是当前实例的本地标识。
	ServiceInstanceId string
	// AuthzVerification 是业务入口必需的本地验签规则；实际方法由拦截器捕获。
	AuthzVerification *service.AuthzSignVerificationOptions
	// AuthzSkipMethods 只精确匹配独立入口，跳过后不建立可信用户或 Invoke 身份。
	AuthzSkipMethods []string
	// TrustedClientIP 必须先验证连接对端再解释代理 metadata；未配置时仅取对端 IP。
	TrustedClientIP func(context.Context, metadata.MD) string
}

// NewServiceContextUnaryInterceptor 在业务 handler 前捕获真实 FullMethod 并构建上下文。
func NewServiceContextUnaryInterceptor(options ServiceContextInterceptorOptions) grpc.UnaryServerInterceptor {
	build := incomingContextBuilder(options)

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info == nil {
			return nil, status.Error(codes.Internal, "missing actual service entry")
		}

		in, err := build(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}

		return handler(in, req)
	}
}

// NewServiceContextStreamInterceptor 将请求视图绑定到整个流生命周期，保留原始取消信号。
func NewServiceContextStreamInterceptor(options ServiceContextInterceptorOptions) grpc.StreamServerInterceptor {
	build := incomingContextBuilder(options)

	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if info == nil {
			return status.Error(codes.Internal, "missing actual service entry")
		}

		in, err := build(stream.Context(), info.FullMethod)
		if err != nil {
			return err
		}

		return handler(srv, &contextServerStream{ServerStream: stream, ctx: in})
	}
}

type contextServerStream struct {
	// ServerStream 保留原始收发及状态接口，仅替换 Context 访问器。
	grpc.ServerStream
	// ctx 是已清洗 metadata 并注入业务视图的派生上下文。
	ctx context.Context
}

func (s *contextServerStream) Context() context.Context { return s.ctx }

// incomingContextBuilder 供 unary/stream 复用相同的精确跳过与绑定策略。
func incomingContextBuilder(options ServiceContextInterceptorOptions) func(context.Context, string) (context.Context, error) {
	skip := make(map[string]bool, len(options.AuthzSkipMethods))
	for _, method := range options.AuthzSkipMethods {
		if method != "" {
			skip[method] = true
		}
	}

	return func(ctx context.Context, method string) (context.Context, error) {
		if method == "" {
			return nil, status.Error(codes.Internal, "missing actual service entry")
		}

		// 漏装配置必须拒绝，独立入口只有精确命中跳过列表才可不验签。
		if !skip[method] && options.AuthzVerification == nil {
			return nil, status.Error(codes.Internal, "authz verification is not configured")
		}

		var ip string

		md, _ := metadata.FromIncomingContext(ctx)
		if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
			ip, _, _ = net.SplitHostPort(p.Addr.String())
		}

		if options.TrustedClientIP != nil {
			// 代理策略只收到副本，不能反向修改原始入站 metadata。
			ip = options.TrustedClientIP(ctx, md.Copy())
		}

		build := service.BuildContextOptions{
			ServiceAppId: options.ServiceAppId, ServiceInstanceId: options.ServiceInstanceId,
			ServiceMethod: constant.RequestMethodGrpcString, ServicePath: method, ClientIp: ip,
		}

		if !skip[method] {
			// 独立入口只跳过验签，不从普通头恢复任何可信身份。
			build.AuthzVerification = options.AuthzVerification
		}

		in, err := service.NewIncomingContext(ctx, md, build)
		if err != nil {
			// 对外使用稳定错误，不把凭证或签名解析细节带入响应。
			return nil, status.Error(codes.Unauthenticated, "invalid service context")
		}

		return in, nil
	}
}
