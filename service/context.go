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
	// UserId 表示用户主体 ID；服务或匿名主体为空。
	UserId string
	// AppId 是调用方应用 ID 的兼容字段，当前与 InvokeAppId 保持一致。
	AppId string
	// TenantId 表示当前主体所属租户 ID。
	TenantId string
	// OrgIds 表示当前主体关联的组织 ID 列表。
	OrgIds []string
	// RoleIds 表示当前主体关联的角色 ID 列表。
	RoleIds []string
	// SubjectType 表示本次请求主体类型：anonymous/user/service。
	SubjectType string
	// InvokeAppId 表示发起调用的应用 ID。
	InvokeAppId string
	// TargetAppId 表示被访问资源所属应用 ID。
	TargetAppId string
	// ResourceType 表示本次授权动作，HTTP 为方法名，gRPC 为 GRPC。
	ResourceType string
	// ResourcePath 表示本次授权资源路径。
	ResourcePath string
	// DecisionId 表示 authz allow 决策 ID。
	DecisionId string
	// AuthzContextToken 保存 authz 注入的原始签名 JWS，便于审计或延迟验签。
	AuthzContextToken string
	// AuthzContext 保存已本地验签通过的可信上下文；未启用验签时为空。
	AuthzContext *AuthzContext
	// TraceId 表示从当前 OTel span 提取的 trace 标识快照，不对应自定义 header。
	TraceId string
	// ServiceAppId 表示当前服务自身的应用 ID。
	ServiceAppId string
	// ServiceInstanceId 表示当前服务自身的实例 ID。
	ServiceInstanceId string
}

// BuildContextOptions 定义构建服务主上下文时需要补齐的服务自身身份。
type BuildContextOptions struct {
	ServiceAppId      string
	ServiceInstanceId string
	// AuthzVerification 配置后，BuildVerifiedContext 会用它校验 x-firefly-authz-context。
	AuthzVerification *AuthzContextVerificationOptions
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
	value := buildContextFromMetadata(ctx, options)

	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		value.TraceId = span.SpanContext().TraceID().String()
	}

	return value
}

// BuildVerifiedContext 从入站 metadata 构造服务主上下文，并校验 authz 签名上下文。
func BuildVerifiedContext(ctx context.Context, options BuildContextOptions) (*Context, error) {
	value := BuildContext(ctx, options)
	if options.AuthzVerification == nil {
		return value, nil
	}

	authzContext, err := VerifyAuthzContext(value.AuthzContextToken, *options.AuthzVerification)
	if err != nil {
		return nil, err
	}
	value.applyVerifiedAuthzContext(authzContext)
	return value, nil
}

// buildContextFromMetadata 只处理 Firefly 标准 metadata 到服务主上下文的字段映射。
func buildContextFromMetadata(ctx context.Context, options BuildContextOptions) *Context {
	value := &Context{
		ServiceAppId:      strings.TrimSpace(options.ServiceAppId),
		ServiceInstanceId: strings.TrimSpace(options.ServiceInstanceId),
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		value.UserId = ParseMetaKey(md, constant.UserId)
		value.InvokeAppId = firstNonEmpty(ParseMetaKey(md, constant.InvokeAppId), ParseMetaKey(md, constant.AppId))
		value.AppId = value.InvokeAppId
		value.TenantId = ParseMetaKey(md, constant.TenantId)
		value.SubjectType = ParseMetaKey(md, constant.SubjectType)
		value.TargetAppId = ParseMetaKey(md, constant.TargetAppId)
		value.ResourceType = ParseMetaKey(md, constant.ResourceType)
		value.ResourcePath = ParseMetaKey(md, constant.ResourcePath)
		value.DecisionId = ParseMetaKey(md, constant.DecisionId)
		value.AuthzContextToken = ParseMetaKey(md, constant.AuthzContext)
		value.OrgIds = cloneStrings(md.Get(constant.OrgIds))
		value.RoleIds = cloneStrings(md.Get(constant.RoleIds))
	}

	return value
}

// applyVerifiedAuthzContext 使用签名上下文覆盖普通 header，避免业务代码信任可伪造字段。
func (c *Context) applyVerifiedAuthzContext(authzContext *AuthzContext) {
	if c == nil || authzContext == nil {
		return
	}
	c.AuthzContext = authzContext
	c.UserId = authzContext.UserId
	c.TenantId = authzContext.TenantId
	c.SubjectType = authzContext.SubjectType
	c.InvokeAppId = authzContext.InvokeAppId
	c.AppId = authzContext.InvokeAppId
	c.TargetAppId = authzContext.TargetAppId
	c.ResourceType = authzContext.ResourceType
	c.ResourcePath = authzContext.ResourcePath
	c.DecisionId = authzContext.DecisionId
}

func ParseMetaKey(md metadata.MD, key string) string {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
