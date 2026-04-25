package gm

import (
	"context"

	"github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc"
)

// ServiceContextInterceptorOptions 定义拦截器选项。
type ServiceContextInterceptorOptions struct {
	ServiceAppId      string
	ServiceInstanceId string
}

// NewServiceContextUnaryInterceptor 在请求入口统一建立并注入 service.Context。
func NewServiceContextUnaryInterceptor(options ServiceContextInterceptorOptions) grpc.UnaryServerInterceptor {
	buildOptions := service.BuildContextOptions{
		ServiceAppId:      options.ServiceAppId,
		ServiceInstanceId: options.ServiceInstanceId,
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		serviceContext := service.BuildContext(ctx, buildOptions)
		if serviceContext != nil {
			ctx = service.WithContext(ctx, serviceContext)
		}

		return handler(ctx, req)
	}
}
