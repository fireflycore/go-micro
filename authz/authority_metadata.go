package authz

import (
	"context"
	"strings"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// PrepareOutgoingAuthorityMetadata 为本次调用构建新白名单，不复制原始 metadata 或上一跳证明。
func PrepareOutgoingAuthorityMetadata(ctx context.Context, target service.OutgoingTarget, provider ServiceAuthorityProvider) (metadata.MD, error) {
	out, err := buildOutgoingAuthorityContext(ctx, target, provider)
	if err != nil {
		return nil, err
	}

	md := out.Metadata()
	// gRPC 普通 metadata 只支持可打印 ASCII；不能等到传输层才将合法 UTF-8 报成 Internal。
	// 不私自增加转义或 -bin 协议，HTTP 适配仍可使用原始 UTF-8 观测值。
	for _, values := range md {
		for _, value := range values {
			for i := 0; i < len(value); i++ {
				if value[i] < 0x20 || value[i] > 0x7e {
					return nil, service.ErrIncomingContextInvalid
				}
			}
		}
	}

	return md, nil
}

// buildOutgoingAuthorityContext 统一获取当前服务凭证，协议编码限制留给各自适配器。
func buildOutgoingAuthorityContext(ctx context.Context, target service.OutgoingTarget, provider ServiceAuthorityProvider) (*service.OutgoingContext, error) {
	if provider == nil {
		return nil, ErrServiceAuthorityFetchMissing
	}

	token, err := provider.ServiceAuthority(ctx)
	if err != nil {
		// 当前服务凭证失败必须中止，不能退化为仅凭用户身份调用。
		return nil, err
	}

	if strings.TrimSpace(token) == "" {
		return nil, ErrServiceAuthorityTokenMissing
	}

	return service.BuildOutgoingContext(ctx, target, token)
}

// NewOutgoingAuthorityContext 保留父级取消、期限与活动 span，只替换当次出站 metadata。
func NewOutgoingAuthorityContext(ctx context.Context, target service.OutgoingTarget, provider ServiceAuthorityProvider) (context.Context, error) {
	md, err := PrepareOutgoingAuthorityMetadata(ctx, target, provider)
	if err != nil {
		return nil, err
	}
	return metadata.NewOutgoingContext(ctx, md), nil
}

// NewServiceAuthorityUnaryClientInterceptor 为非 invocation 的 unary 调用提供相同的出站边界。
func NewServiceAuthorityUnaryClientInterceptor(provider ServiceAuthorityProvider) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		out, err := NewOutgoingAuthorityContext(ctx, service.OutgoingTarget{TargetMethod: constant.RequestMethodGrpcString, TargetPath: method}, provider)
		if err != nil {
			return err
		}
		return invoker(out, method, req, reply, cc, opts...)
	}
}

// NewServiceAuthorityStreamClientInterceptor 在创建流时独立构建本跳凭证和传播字段。
func NewServiceAuthorityStreamClientInterceptor(provider ServiceAuthorityProvider) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		out, err := NewOutgoingAuthorityContext(ctx, service.OutgoingTarget{TargetMethod: constant.RequestMethodGrpcString, TargetPath: method}, provider)
		if err != nil {
			return nil, err
		}
		return streamer(out, desc, cc, method, opts...)
	}
}
