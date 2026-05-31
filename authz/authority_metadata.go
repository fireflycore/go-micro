package authz

import (
	"context"

	"github.com/fireflycore/go-micro/constant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// PrepareOutgoingAuthorityMetadata 清理旧身份上下文，并写入当前这一跳 service authority。
func PrepareOutgoingAuthorityMetadata(ctx context.Context, md metadata.MD, provider ServiceAuthorityProvider) (metadata.MD, error) {
	// metadata 为空时创建一份新的容器，保证后续 Set/Delete 都安全。
	if md == nil {
		md = metadata.New(nil)
	} else {
		// 调用方传入的 metadata 可能还会被复用，这里复制后再修改。
		md = md.Copy()
	}

	// 出站调用不能继续携带上一跳 authz 注入的普通上下文或签名上下文。
	removeStaleAuthzMetadata(md)

	// 未配置 provider 时只做清理，不强制失败，便于本地或迁移期逐步接入。
	if provider == nil {
		return md, nil
	}

	// 每一跳都重新获取当前服务的 service authority。
	token, err := provider.ServiceAuthority(ctx)
	if err != nil {
		return nil, err
	}
	// provider 返回空 token 时等同于身份不可用。
	if token == "" {
		return nil, ErrServiceAuthorityTokenMissing
	}
	// 当前服务身份必须覆盖继承自上游的 service authority。
	md.Set(constant.ServiceAuthority, token)
	// 返回已经清理并注入当前服务身份的 metadata。
	return md, nil
}

// NewOutgoingAuthorityContext 基于当前 ctx 构造带 Firefly authority 的 gRPC 出站 ctx。
func NewOutgoingAuthorityContext(ctx context.Context, provider ServiceAuthorityProvider) (context.Context, error) {
	// 优先复用已有 outgoing metadata，其次复用 server incoming metadata。
	md := ContextAuthorityMetadata(ctx)
	// 清理旧上下文并注入当前服务 authority。
	prepared, err := PrepareOutgoingAuthorityMetadata(ctx, md, provider)
	if err != nil {
		return nil, err
	}
	// 将处理后的 metadata 写入 outgoing context。
	return metadata.NewOutgoingContext(ctx, prepared), nil
}

// NewServiceAuthorityUnaryClientInterceptor 创建统一的 service authority gRPC 客户端拦截器。
func NewServiceAuthorityUnaryClientInterceptor(provider ServiceAuthorityProvider) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// 先构造新的 outgoing context，再进入真实 gRPC 调用。
		outCtx, err := NewOutgoingAuthorityContext(ctx, provider)
		if err != nil {
			return err
		}
		// 带着 Firefly authority 双头继续执行调用。
		return invoker(outCtx, method, req, reply, cc, opts...)
	}
}

// ContextAuthorityMetadata 复制当前 ctx 中最适合作为出站基础的 metadata。
func ContextAuthorityMetadata(ctx context.Context) metadata.MD {
	// 空 context 直接返回空 metadata。
	if ctx == nil {
		return metadata.New(nil)
	}
	// 已经处于客户端链路时优先复用 outgoing metadata。
	if outgoing, ok := metadata.FromOutgoingContext(ctx); ok {
		return outgoing.Copy()
	}
	// 服务端处理请求后发起下游调用时，复用 incoming metadata。
	if incoming, ok := metadata.FromIncomingContext(ctx); ok {
		return incoming.Copy()
	}
	// 没有任何 metadata 时返回空容器。
	return metadata.New(nil)
}

func removeStaleAuthzMetadata(md metadata.MD) {
	// Authorization 是外部兼容头，不作为 Firefly current 身份入口。
	md.Delete(constant.Authorization)
	// 上一跳 authz 签名上下文只对上一跳 route 有效，不能透传到下一跳。
	md.Delete(constant.AuthzContext)
	// 普通 UserContext 字段应由下一跳 authz 重新注入，不能沿用上一跳的可伪造 header。
	md.Delete(constant.UserId)
	md.Delete(constant.AppId)
	md.Delete(constant.TenantId)
	md.Delete(constant.OrgIds)
	md.Delete(constant.RoleIds)
	// 普通授权事实字段与上一跳 route 绑定，必须由下一跳 ext_authz 重新计算。
	md.Delete(constant.SubjectType)
	md.Delete(constant.InvokeAppId)
	md.Delete(constant.TargetAppId)
	md.Delete(constant.ResourceType)
	md.Delete(constant.ResourcePath)
	md.Delete(constant.DecisionId)
	// 上游服务身份不能透传，后续会由当前服务重新覆盖。
	md.Delete(constant.ServiceAuthority)
}
