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
		md, _ := metadata.FromIncomingContext(ctx)
		pm := md.Copy()

		if _, err := rpc.ParseMetaKey(md, constant.TraceId); err != nil {
			pm.Set(constant.TraceId, uuid.Must(uuid.NewV7()).String())
		}
		if spanId, err := rpc.ParseMetaKey(md, constant.SpanId); err == nil {
			pm.Set(constant.ParentId, spanId)
		}
		pm.Set(constant.SpanId, uuid.Must(uuid.NewV7()).String())

		oc := metadata.NewOutgoingContext(ctx, md.Copy())
		return handler(oc, req)
	}
}

// NewInjectServiceContext 将服务的信息注入到上下文中
func NewInjectServiceContext(conf conf.BootstrapConf) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(rpc.SetRemoteInvokeServiceBeforeContext(ctx, conf), req)
	}
}
