package rpc

import (
	"context"
	"github.com/fireflycore/go-micro/constant"
	"google.golang.org/grpc/metadata"
)

// SetRemoteInvokeServiceBeforeContext 设置远程调用前置上下文
func SetRemoteInvokeServiceBeforeContext(ctx context.Context, appId, endpoint, token string) context.Context {
	pm, _ := metadata.FromIncomingContext(ctx)

	md := pm.Copy()
	md.Set(constant.InvokeServiceAuth, token)
	md.Set(constant.InvokeServiceAppId, appId)
	md.Set(constant.InvokeServiceEndpoint, endpoint)

	return metadata.NewOutgoingContext(ctx, md)
}

// SetRemoteInvokeServiceAfterContext 设置远程调用后置上下文
func SetRemoteInvokeServiceAfterContext(ctx context.Context, appId, endpoint string) context.Context {
	pm, _ := metadata.FromIncomingContext(ctx)

	md := pm.Copy()
	md.Set(constant.TargetServiceAppId, appId)
	md.Set(constant.TargetServiceEndpoint, endpoint)

	return metadata.NewOutgoingContext(ctx, md)
}
