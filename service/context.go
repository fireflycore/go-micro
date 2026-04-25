package service

import (
	"context"
	"strings"

	"github.com/fireflycore/go-micro/constant"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

type contextKey string

const (
	serviceContextValueKey contextKey = "service.context"
)

// Context 表示当前请求在服务内部流转时的统一主上下文。
type Context struct {
	UserId   string
	AppId    string
	TenantId string
	OrgIds   []string
	RoleIds  []string
	// TraceId 表示从当前 OTel span 提取的 trace 标识快照，不对应自定义 header。
	TraceId           string
	RouteMethod       string
	AccessMethod      string
	ServiceAppId      string
	ServiceInstanceId string
}

// BuildContextOptions 定义构建服务主上下文时需要补齐的服务自身身份。
type BuildContextOptions struct {
	ServiceAppId      string
	ServiceInstanceId string
}

// WithContext 将服务主上下文注入到 ctx。
func WithContext(ctx context.Context, value *Context) context.Context {
	if ctx == nil || value == nil {
		return ctx
	}
	return context.WithValue(ctx, serviceContextValueKey, value)
}

// FromContext 从 ctx 读取服务主上下文。
func FromContext(ctx context.Context) (*Context, bool) {
	if ctx == nil {
		return nil, false
	}
	value, ok := ctx.Value(serviceContextValueKey).(*Context)
	return value, ok
}

// MustFromContext 从 ctx 读取服务主上下文，不存在时 panic。
func MustFromContext(ctx context.Context) *Context {
	value, ok := FromContext(ctx)
	if !ok {
		panic("service: context not found in context")
	}
	return value
}

// BuildContext 从入站 metadata 与运行时信息构造服务主上下文。
//
// 它只负责把服务端入口已经拿到的 metadata 与 OTel span 信息结构化，
// 不负责缓存 transport 原文，也不参与出站调用语义。
func BuildContext(ctx context.Context, options BuildContextOptions) *Context {
	value := &Context{
		ServiceAppId:      strings.TrimSpace(options.ServiceAppId),
		ServiceInstanceId: strings.TrimSpace(options.ServiceInstanceId),
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		value.UserId = parseMetaKey(md, constant.UserId)
		value.AppId = parseMetaKey(md, constant.AppId)
		value.TenantId = parseMetaKey(md, constant.TenantId)
		value.RouteMethod = parseMetaKey(md, constant.RouteMethod)
		value.AccessMethod = parseMetaKey(md, constant.AccessMethod)
		value.OrgIds = cloneStrings(md.Get(constant.OrgIds))
		value.RoleIds = cloneStrings(md.Get(constant.RoleIds))
	}

	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		value.TraceId = span.SpanContext().TraceID().String()
	}

	return value
}

func parseMetaKey(md metadata.MD, key string) string {
	if md == nil {
		return ""
	}
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}
