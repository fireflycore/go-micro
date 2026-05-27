package gm

import (
	"context"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ServiceContextInterceptorOptions 定义拦截器选项。
type ServiceContextInterceptorOptions struct {
	ServiceAppId      string
	ServiceInstanceId string
	// AuthzVerification 非空时，入口会本地验签 x-firefly-authz-context。
	AuthzVerification *service.AuthzContextVerificationOptions
	// AuthzSkipMethods 表示不执行 authz 上下文验签的 gRPC 完整方法名，常用于 health check。
	AuthzSkipMethods []string
}

// NewServiceContextUnaryInterceptor 在请求入口统一建立并注入 service.Context。
func NewServiceContextUnaryInterceptor(options ServiceContextInterceptorOptions) grpc.UnaryServerInterceptor {
	buildOptions := service.BuildContextOptions{
		ServiceAppId:      options.ServiceAppId,
		ServiceInstanceId: options.ServiceInstanceId,
	}
	skipAuthzMethods := buildServiceContextAuthzSkipMethods(options.AuthzSkipMethods)

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		serviceContext, err := buildServiceContext(ctx, info, buildOptions, options.AuthzVerification, skipAuthzMethods)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		if serviceContext != nil {
			ctx = service.WithContext(ctx, serviceContext)
		}

		return handler(ctx, req)
	}
}

func buildServiceContext(ctx context.Context, info *grpc.UnaryServerInfo, buildOptions service.BuildContextOptions, verification *service.AuthzContextVerificationOptions, skipAuthzMethods map[string]struct{}) (*service.Context, error) {
	if verification == nil || shouldSkipServiceContextAuthz(info, skipAuthzMethods) {
		return service.BuildContext(ctx, buildOptions), nil
	}

	// 每次请求复制验签选项，避免把当前 gRPC 方法推导出的期望值写回共享配置。
	resolvedVerification := *verification
	if resolvedVerification.ExpectedTargetAppId == "" {
		resolvedVerification.ExpectedTargetAppId = buildOptions.ServiceAppId
	}
	if resolvedVerification.ExpectedResourceType == "" {
		resolvedVerification.ExpectedResourceType = constant.RequestMethodGrpcString
	}
	if resolvedVerification.ExpectedResourcePath == "" && info != nil {
		resolvedVerification.ExpectedResourcePath = info.FullMethod
	}

	buildOptions.AuthzVerification = &resolvedVerification
	return service.BuildVerifiedContext(ctx, buildOptions)
}

func buildServiceContextAuthzSkipMethods(methods []string) map[string]struct{} {
	if len(methods) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		if method == "" {
			continue
		}
		result[method] = struct{}{}
	}
	return result
}

func shouldSkipServiceContextAuthz(info *grpc.UnaryServerInfo, methods map[string]struct{}) bool {
	if len(methods) == 0 || info == nil {
		return false
	}
	_, ok := methods[info.FullMethod]
	return ok
}
