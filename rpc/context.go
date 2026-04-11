package rpc

import (
	"context"
	"fmt"
	"time"

	"github.com/fireflycore/go-micro/config"
	"github.com/fireflycore/go-micro/constant"
	"google.golang.org/grpc/metadata"
)

// ServiceContext 持有服务级别的静态元信息，用于构造服务间调用的出站上下文。
// 应在服务启动时初始化一次，作为单例注入或封装进中间件使用。
type ServiceContext struct {
	metadata metadata.MD

	bootstrapConf config.BootstrapConfig
}

// NewServiceContext 初始化服务上下文。
// 基于启动配置构建服务级别的静态 metadata，后续每次远程调用都会以此为基础进行扩展。
func NewServiceContext(bootstrapConf config.BootstrapConfig) *ServiceContext {
	ist := &ServiceContext{
		bootstrapConf: bootstrapConf,
	}

	ist.metadata = ist.BuildServiceMetadata()

	return ist
}

// GetMetadata 返回服务静态元信息的副本。
func (sc *ServiceContext) GetMetadata() metadata.MD {
	return sc.metadata.Copy()
}

// BuildServiceMetadata 构建服务级别的静态元信息。
func (sc *ServiceContext) BuildServiceMetadata() metadata.MD {
	md := metadata.MD{}

	// 路由标识，区分服务调用与终端用户调用
	md.Set(constant.RouteMethod, constant.RouteMethodService)

	// 服务身份，用于下游鉴权
	md.Set(constant.InvokeServiceAppId, sc.bootstrapConf.GetAppId())
	md.Set(constant.InvokeServiceEndpoint, sc.bootstrapConf.GetServiceEndpoint())
	md.Set(constant.InvokeServiceAuth, sc.bootstrapConf.GetServiceAuthToken())

	md.Set(constant.ClientType, fmt.Sprint(constant.ClientTypeServer))
	md.Set(constant.ClientName, sc.bootstrapConf.GetAppName())
	md.Set(constant.ClientVersion, sc.bootstrapConf.GetAppVersion())

	md.Set(constant.SystemType, fmt.Sprint(sc.bootstrapConf.GetSystemType()))
	md.Set(constant.SystemName, sc.bootstrapConf.GetSystemName())
	md.Set(constant.SystemVersion, sc.bootstrapConf.GetSystemVersion())

	return md
}

// NewOutgoingContext 将 metadata 写入新的出站上下文，并附加超时控制。
func (sc *ServiceContext) NewOutgoingContext(md metadata.MD, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	ctx = metadata.NewOutgoingContext(ctx, md)
	return ctx, cancel
}

// NewOutgoingContextFrom 基于 base 创建出站上下文，并附加 metadata 与超时控制。
func (sc *ServiceContext) NewOutgoingContextFrom(base context.Context, md metadata.MD, timeout time.Duration) (context.Context, context.CancelFunc) {
	if base == nil {
		base = context.Background()
	}
	ctx := metadata.NewOutgoingContext(context.WithoutCancel(base), md)
	return context.WithTimeout(ctx, timeout)
}

// MergeServiceMetadata 将本服务的静态元信息合并进目标 md。
// RouteMethod 已存在时不覆盖，其余字段一律以服务静态值为准。
func (sc *ServiceContext) MergeServiceMetadata(md metadata.MD) metadata.MD {
	for k, v := range sc.metadata.Copy() {
		if k == constant.RouteMethod && len(md.Get(constant.RouteMethod)) != 0 {
			continue
		}
		md.Set(k, v...)
	}
	return md
}

// WithPureContext 创建一个与调用方请求完全隔离的纯净出站上下文。
func (sc *ServiceContext) WithPureContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return sc.NewOutgoingContext(sc.GetMetadata(), timeout)
}

// WithExternalContext 将外部传入的 metadata 与本服务静态元信息合并，构造出站上下文。
func (sc *ServiceContext) WithExternalContext(md metadata.MD, timeout time.Duration) (context.Context, context.CancelFunc) {
	if md == nil {
		md = metadata.MD{}
	} else {
		md = md.Copy()
	}
	return sc.NewOutgoingContext(sc.MergeServiceMetadata(md), timeout)
}

// WithInheritContext 在父上下文的基础上，创建携带完整链路信息的出站上下文。
func (sc *ServiceContext) WithInheritContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	omd, ok := metadata.FromOutgoingContext(parent)
	if !ok {
		imd, _ := metadata.FromIncomingContext(parent)
		omd = imd
	}

	md := omd.Copy()

	md = sc.MergeServiceMetadata(md)

	return sc.NewOutgoingContextFrom(parent, md, timeout)
}
