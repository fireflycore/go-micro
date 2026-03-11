package rpc

import (
	"context"
	"fmt"
	"time"

	"github.com/fireflycore/go-micro/conf"
	"github.com/fireflycore/go-micro/constant"
	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

// ServiceContext 持有服务级别的静态元信息，用于构造服务间调用的出站上下文。
// 应在服务启动时初始化一次，作为单例注入或封装进中间件使用。
type ServiceContext struct {
	metadata metadata.MD

	bootstrapConf conf.BootstrapConf
}

// NewServiceContext 初始化服务上下文。
// 基于启动配置构建服务级别的静态 metadata，后续每次远程调用都会以此为基础进行扩展。
func NewServiceContext(bootstrapConf conf.BootstrapConf) *ServiceContext {
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

// InjectTrace 将链路追踪字段注入 metadata，维护 TraceId -> ParentId -> SpanId 的调用链层级。
//
// 规则：
//   - TraceId：有则继承（保持链路唯一性），无则新建（标识链路起点）
//   - ParentId：将上游的 SpanId 记录为本跳的 ParentId，构建调用树
//   - SpanId：每次调用都生成新值，唯一标识当前这一跳
func (sc *ServiceContext) InjectTrace(md metadata.MD) metadata.MD {
	if _, err := ParseMetaKey(md, constant.TraceId); err != nil {
		// 没有 TraceId，说明是链路起点，生成新的
		md.Set(constant.TraceId, uuid.Must(uuid.NewV7()).String())
	}
	if spanId, err := ParseMetaKey(md, constant.SpanId); err == nil {
		// 将上游 SpanId 记录为本跳的 ParentId
		md.Set(constant.ParentId, spanId)
	}
	// 为本次调用生成新的 SpanId
	md.Set(constant.SpanId, uuid.Must(uuid.NewV7()).String())

	return md
}

// NewOutgoingContext 将 metadata 写入新的出站上下文，并附加超时控制。
func (sc *ServiceContext) NewOutgoingContext(md metadata.MD, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	ctx = metadata.NewOutgoingContext(ctx, md)
	return ctx, cancel
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
//
// 适用场景：
//   - 定时任务、事件驱动等服务主动发起的后台调用
//   - 无需透传用户身份的内部服务调用
//
// 行为说明：
//   - 基于 context.Background() 创建，调用方的取消信号不会传播，生命周期由 timeout 独立控制
//   - 不携带用户信息（UserId / AppId / TenantId）
//   - 自动生成全新的 TraceId 和 SpanId，作为新链路的起点
func (sc *ServiceContext) WithPureContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	md := sc.GetMetadata()
	md = sc.InjectTrace(md)
	return sc.NewOutgoingContext(md, timeout)
}

// WithExternalContext 将外部传入的 metadata 与本服务静态元信息合并，构造出站上下文。
//
// 适用场景：
//   - 处理来自消息队列、Webhook 等非 gRPC 入口的请求，已有部分上下文信息需要透传
//   - 由外部系统注入初始 metadata，服务侧补全自身身份信息后继续向下游调用
//
// 行为说明：
//   - 以传入的 md 为基础，本服务静态元信息作为补充（RouteMethod 不覆盖）
//   - 自动注入新的 TraceId（若不存在）和 SpanId，维护链路连续性
//   - 生命周期由 timeout 独立控制，与调用方上下文无关
func (sc *ServiceContext) WithExternalContext(md metadata.MD, timeout time.Duration) (context.Context, context.CancelFunc) {
	md = sc.MergeServiceMetadata(md)
	md = sc.InjectTrace(md)
	return sc.NewOutgoingContext(md, timeout)
}

// WithInheritContext 在父上下文的基础上，创建携带完整链路信息的出站上下文。
//
// 适用场景：
//   - 处理用户请求时，服务需要继续调用下游服务
//   - 需要保持 trace 链路连续，并透传用户身份
//
// 行为说明：
//   - 基于 context.Background() 创建，生命周期由 timeout 独立控制，不受父上下文取消影响
//   - 继承父上下文中的 TraceId，更新 SpanId 并将原 SpanId 设为 ParentId
//   - 透传父上下文中的用户信息（UserId / AppId / TenantId）
//   - 合并本服务的静态元信息，下游可据此识别直接调用方
func (sc *ServiceContext) WithInheritContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	imd, _ := metadata.FromIncomingContext(parent)
	md := imd.Copy()

	md = sc.MergeServiceMetadata(md)
	md = sc.InjectTrace(md)

	return sc.NewOutgoingContext(md, timeout)
}
