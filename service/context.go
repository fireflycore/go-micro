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
	// ServiceAppId 表示当前进程所属应用 ID，用于服务自身身份和 target_app_id 默认校验。
	ServiceAppId string
	// ServiceInstanceId 表示当前进程实例 ID，用于日志、审计和实例排障。
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
	// 先从 gRPC metadata 读取 Firefly 标准上下文字段。
	value := buildContextFromMetadata(ctx, options)

	// 再从当前 OTel span 取 trace_id 快照，避免重新定义自有 trace header。
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		// TraceId 只进入服务内上下文和日志，不参与跨服务传播。
		value.TraceId = span.SpanContext().TraceID().String()
	}

	// 返回未验签版本；调用方若需要信任根，应改用 BuildVerifiedContext。
	return value
}

// BuildVerifiedContext 从入站 metadata 构造服务主上下文，并校验 authz 签名上下文。
func BuildVerifiedContext(ctx context.Context, options BuildContextOptions) (*Context, error) {
	// 先按普通 header 构造上下文，保留未验签时的读取便利。
	value := BuildContext(ctx, options)
	// 未配置验签时保持历史行为，只做结构化，不阻断请求。
	if options.AuthzVerification == nil {
		return value, nil
	}

	// 使用 authz 注入的 JWS 作为服务侧信任根。
	authzContext, err := VerifyAuthzContext(value.AuthzContextToken, *options.AuthzVerification)
	if err != nil {
		// 验签失败直接返回错误，由入口中间件转换成 gRPC 未认证错误。
		return nil, err
	}
	// 验签通过后，用签名 claim 覆盖普通 header，避免信任客户端伪造字段。
	value.applyVerifiedAuthzContext(authzContext)
	// 返回已经绑定可信 authz 上下文的 ServiceContext。
	return value, nil
}

// buildContextFromMetadata 只处理 Firefly 标准 metadata 到服务主上下文的字段映射。
func buildContextFromMetadata(ctx context.Context, options BuildContextOptions) *Context {
	// 先注入当前服务自身身份，这两个字段不来自调用方 metadata。
	value := &Context{
		ServiceAppId:      strings.TrimSpace(options.ServiceAppId),
		ServiceInstanceId: strings.TrimSpace(options.ServiceInstanceId),
	}

	// 只有 gRPC 入站 metadata 存在时才解析调用方上下文。
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// UserId 来自 authz allow 后注入的普通 header；服务主体通常为空。
		value.UserId = ParseMetaKey(md, constant.UserId)
		// InvokeAppId 是新语义，AppId 是存量业务兼容字段，因此优先读 InvokeAppId。
		value.InvokeAppId = firstNonEmpty(ParseMetaKey(md, constant.InvokeAppId), ParseMetaKey(md, constant.AppId))
		// AppId 保持为调用方应用 ID，避免存量业务读取 AppId 时语义漂移。
		value.AppId = value.InvokeAppId
		// TenantId 表示主体租户，服务或公共接口可能为空或通配。
		value.TenantId = ParseMetaKey(md, constant.TenantId)
		// SubjectType 区分 anonymous/user/service，替代旧 route-method。
		value.SubjectType = ParseMetaKey(md, constant.SubjectType)
		// TargetAppId 表示当前被访问资源所属 app_id。
		value.TargetAppId = ParseMetaKey(md, constant.TargetAppId)
		// ResourceType 是 Casbin action，HTTP 为方法名，gRPC 为 GRPC。
		value.ResourceType = ParseMetaKey(md, constant.ResourceType)
		// ResourcePath 是 Casbin object，HTTP 为 path，gRPC 为 FullMethod。
		value.ResourcePath = ParseMetaKey(md, constant.ResourcePath)
		// DecisionId 用于把业务日志和 authz allow 决策关联起来。
		value.DecisionId = ParseMetaKey(md, constant.DecisionId)
		// AuthzContextToken 保存原始 JWS，后续 BuildVerifiedContext 会用它验签。
		value.AuthzContextToken = ParseMetaKey(md, constant.AuthzContext)
		// OrgIds 可能有多个 metadata value，必须复制为服务上下文独占切片。
		value.OrgIds = cloneStrings(md.Get(constant.OrgIds))
		// RoleIds 同样复制，避免调用方后续修改 metadata 影响上下文。
		value.RoleIds = cloneStrings(md.Get(constant.RoleIds))
	}

	// 返回只完成结构化的上下文，是否可信由调用方选择是否验签决定。
	return value
}

// applyVerifiedAuthzContext 使用签名上下文覆盖普通 header，避免业务代码信任可伪造字段。
func (c *Context) applyVerifiedAuthzContext(authzContext *AuthzContext) {
	// 空 receiver 或空 claim 都直接返回，保持方法幂等和空安全。
	if c == nil || authzContext == nil {
		return
	}
	// 保存完整可信 claim，业务或日志需要审计细节时可读取。
	c.AuthzContext = authzContext
	// UserId 以签名 claim 为准，防止客户端伪造普通 x-firefly-user-id。
	c.UserId = authzContext.UserId
	// TenantId 以签名 claim 为准。
	c.TenantId = authzContext.TenantId
	// SubjectType 以签名 claim 为准。
	c.SubjectType = authzContext.SubjectType
	// InvokeAppId 以签名 claim 为准。
	c.InvokeAppId = authzContext.InvokeAppId
	// AppId 是兼容字段，也同步为可信调用方 app_id。
	c.AppId = authzContext.InvokeAppId
	// TargetAppId 以签名 claim 为准。
	c.TargetAppId = authzContext.TargetAppId
	// ResourceType 以签名 claim 为准。
	c.ResourceType = authzContext.ResourceType
	// ResourcePath 以签名 claim 为准。
	c.ResourcePath = authzContext.ResourcePath
	// DecisionId 以签名 claim 为准。
	c.DecisionId = authzContext.DecisionId
}

func ParseMetaKey(md metadata.MD, key string) string {
	// metadata 为空时直接返回空字符串，调用方无需重复判空。
	if md == nil {
		return ""
	}
	// gRPC metadata 同一个 key 可以有多个值，这里只读取约定的第一个值。
	values := md.Get(key)
	// key 不存在时返回空字符串，保持 BuildContext 的字段默认零值。
	if len(values) == 0 {
		return ""
	}
	// 返回第一个 metadata value，调用方负责决定是否 trim 或 fallback。
	return values[0]
}

func cloneStrings(values []string) []string {
	// 没有值时返回 nil，避免制造无意义空切片。
	if len(values) == 0 {
		return nil
	}
	// 复制切片，避免 ServiceContext 共享 metadata 内部底层数组。
	return append([]string(nil), values...)
}

func firstNonEmpty(values ...string) string {
	// 按优先级逐个检查候选值，常用于新字段到兼容字段的 fallback。
	for _, value := range values {
		// 先裁剪空白，再判断是否可用，避免把空格当成有效 app_id。
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	// 所有候选都为空时返回空字符串。
	return ""
}
