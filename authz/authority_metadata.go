package authz

import (
	"context"
	"strings"

	"github.com/fireflycore/go-micro/constant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var outgoingAuthorityMetadataAllowlist = map[string]struct{}{
	// UserAuthority 是用户 authority 原文，需要贯穿完整调用链交给下一跳 authz 校验。
	constant.UserAuthority: {},
	// AuthzSign 是 authz 签名过的短 TTL payload，只能辅助下一跳 authz 复用身份解析结果，不能复用上一跳授权结论。
	constant.AuthzSign: {},
	// OTel/W3C 传播头由当前链路继续传递，保证 trace 不被出站清理截断。
	constant.TraceParent: {},
	constant.TraceState:  {},
	constant.Baggage:     {},
	// 客户端和入口代理事实用于下游访问日志，不参与权限判定。
	constant.XRealIp:       {},
	constant.XForwardedFor: {},
	constant.AppLanguage:   {},
	constant.AppVersion:    {},
	constant.SystemType:    {},
	constant.SystemName:    {},
	constant.SystemVersion: {},
	constant.ClientType:    {},
	constant.ClientName:    {},
	constant.ClientVersion: {},
}

// PrepareOutgoingAuthorityMetadata 清理出站 metadata，并写入当前这一跳 service authority。
//
// 这里处理的是传输层 metadata，不处理 service.Context / AuthzSign 这类进程内结构。
func PrepareOutgoingAuthorityMetadata(ctx context.Context, md metadata.MD, provider ServiceAuthorityProvider) (metadata.MD, error) {
	// 先按业务服务出站白名单重建 metadata，清理上一跳普通上下文字段和未知 header。
	md = filterOutgoingAuthorityMetadata(md)

	// 未配置 provider 时只做清理，仅适合获取 service token 的启动链路、authz 这类无下游热路径组件或测试链路。
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

func filterOutgoingAuthorityMetadata(md metadata.MD) metadata.MD {
	// nil metadata 直接返回空容器，保证调用方后续 Set 安全。
	if md == nil {
		return metadata.New(nil)
	}
	// 使用新 map 承载白名单字段，避免在原 map 上做增删造成调用方可见副作用。
	filtered := metadata.New(nil)
	for key, values := range md {
		// gRPC metadata key 语义上大小写不敏感，统一小写后再匹配白名单。
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		// 不在白名单中的字段一律丢弃，包括普通身份 metadata、上一跳 service authority 和未知业务 header。
		if _, ok := outgoingAuthorityMetadataAllowlist[normalizedKey]; !ok {
			continue
		}
		// 复制 value 切片，避免调用方后续修改原 metadata 影响本次出站调用。
		filtered[normalizedKey] = append(filtered[normalizedKey], values...)
	}
	// 返回只包含允许透传字段的新 metadata。
	return filtered
}
