package gm

import (
	"context"

	"github.com/fireflycore/go-micro/conf"
	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/rpc"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// NewBeforeGuard 前置守卫
func NewBeforeGuard() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		pm, _ := metadata.FromIncomingContext(ctx)

		md := pm.Copy()
		if _, err := rpc.ParseMetaKey(pm, constant.TraceId); err != nil {
			md.Set(constant.TraceId, uuid.Must(uuid.NewV7()).String())
		}
		if spanId, err := rpc.ParseMetaKey(pm, constant.SpanId); err == nil {
			md.Set(constant.ParentId, spanId)
		}
		md.Set(constant.SpanId, uuid.Must(uuid.NewV7()).String())

		return handler(metadata.NewIncomingContext(ctx, md), req)
	}
}

// NewAfterGuard 后置守卫
func NewAfterGuard() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		md, _ := metadata.FromIncomingContext(ctx)

		// 可根据需要添加其他逻辑

		return handler(metadata.NewOutgoingContext(ctx, md.Copy()), req)
	}
}

// NewInjectServiceContext 将服务的信息注入到上下文中
func NewInjectServiceContext(conf conf.BootstrapConf) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(rpc.SetRemoteInvokeServiceBeforeContext(ctx, conf), req)
	}
}
