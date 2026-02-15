package gm

import (
	"context"

	"github.com/fireflycore/go-micro/conf"
	"github.com/fireflycore/go-micro/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// PropagateIncomingMetadata 将入站元数据传播到出站上下文中
func PropagateIncomingMetadata(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	// 复制一份元数据到 OutgoingContext，保证服务内的下游 gRPC 调用能自动携带同一套上下文信息，
	// 同时避免对原始入站元数据的意外修改。
	oc := metadata.NewOutgoingContext(ctx, md.Copy())
	return handler(oc, req)
}

// NewInjectServiceContext 将服务的信息注入到上下文中
func NewInjectServiceContext(conf conf.BootstrapConf) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(rpc.SetRemoteInvokeServiceBeforeContext(ctx, conf), req)
	}
}
