package rpc

import (
	"context"
	"fmt"

	"github.com/fireflycore/go-micro/conf"
	"github.com/fireflycore/go-micro/constant"
	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

// SetRemoteInvokeServiceBeforeContext 设置远程调用前置上下文
func SetRemoteInvokeServiceBeforeContext(ctx context.Context, bootstrapConf conf.BootstrapConf) context.Context {
	pm, _ := metadata.FromIncomingContext(ctx)

	md := pm.Copy()
	md.Set(constant.InvokeServiceAppId, bootstrapConf.GetAppId())
	md.Set(constant.InvokeServiceEndpoint, bootstrapConf.GetServiceEndpoint())
	md.Set(constant.InvokeServiceAuth, bootstrapConf.GetServiceAuthToken())

	md.Set(constant.ClientType, fmt.Sprint(constant.ClientTypeServer))
	md.Set(constant.ClientName, bootstrapConf.GetAppName())
	md.Set(constant.ClientVersion, bootstrapConf.GetAppVersion())

	md.Set(constant.SystemType, fmt.Sprint(bootstrapConf.GetSystemType()))
	md.Set(constant.SystemName, bootstrapConf.GetSystemName())
	md.Set(constant.SystemVersion, bootstrapConf.GetSystemVersion())

	if _, err := ParseMetaKey(md, constant.TraceId); err != nil {
		md.Set(constant.TraceId, uuid.New().String())
	}

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
