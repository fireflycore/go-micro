package rpc

import (
	"context"
	"fmt"

	"github.com/fireflycore/go-micro/conf"
	"github.com/fireflycore/go-micro/constant"
	"google.golang.org/grpc/metadata"
)

// NewRemoteInvokeServiceContext 初始化远程调用服务上下文, 适用于服务自身（无其他用户特征）调用其他服务时，可基于此上下文进行设置上下文
func NewRemoteInvokeServiceContext(bootstrapConf conf.BootstrapConf) context.Context {
	return SetRemoteInvokeServiceBeforeContext(context.Background(), bootstrapConf)
}

// SetRemoteInvokeServiceBeforeContext 设置远程调用前置上下文，基于当前上下文进行设置服务自身的信息，并设置链路追踪，一般是将其封装成中间件使用
func SetRemoteInvokeServiceBeforeContext(ctx context.Context, bootstrapConf conf.BootstrapConf) context.Context {
	pm, _ := metadata.FromIncomingContext(ctx)

	md := pm.Copy()
	md.Set(constant.RouteMethod, constant.RouteMethodService)
	md.Set(constant.InvokeServiceAppId, bootstrapConf.GetAppId())
	md.Set(constant.InvokeServiceEndpoint, bootstrapConf.GetServiceEndpoint())
	md.Set(constant.InvokeServiceAuth, bootstrapConf.GetServiceAuthToken())

	md.Set(constant.ClientType, fmt.Sprint(constant.ClientTypeServer))
	md.Set(constant.ClientName, bootstrapConf.GetAppName())
	md.Set(constant.ClientVersion, bootstrapConf.GetAppVersion())

	md.Set(constant.SystemType, fmt.Sprint(bootstrapConf.GetSystemType()))
	md.Set(constant.SystemName, bootstrapConf.GetSystemName())
	md.Set(constant.SystemVersion, bootstrapConf.GetSystemVersion())

	return metadata.NewOutgoingContext(ctx, md)
}

// SetRemoteInvokeServiceAfterContext 设置远程调用后置上下文，一般是grpc-gateway使用的
func SetRemoteInvokeServiceAfterContext(ctx context.Context, appId, endpoint string) context.Context {
	pm, _ := metadata.FromIncomingContext(ctx)

	md := pm.Copy()
	md.Set(constant.TargetServiceAppId, appId)
	md.Set(constant.TargetServiceEndpoint, endpoint)

	return metadata.NewOutgoingContext(ctx, md)
}
