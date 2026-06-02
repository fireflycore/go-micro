package service

import (
	"context"

	"github.com/fireflycore/go-micro/constant"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

type contextKey string

const (
	serviceContextValueKey contextKey = "service.context"
)

// Context 表示当前请求在服务进程内流转时的统一主上下文。
//
// 它不是跨进程传输对象；跨进程只传 HTTP header / gRPC metadata。
// Context 由入口 metadata、当前服务配置和可选的 x-firefly-authz-sign 验签结果组装而来。
type Context struct {
	// AppLanguage 表示客户端应用语言偏好。
	AppLanguage string
	// Session 表示 authz 从用户 token 中解析出的会话标识。
	Session string
	// UserId 表示用户主体 ID；服务或匿名主体为空。
	UserId string
	// AppId 表示用户身份中的应用 ID；没有用户身份时为空。
	AppId string
	// TenantId 表示当前主体所属租户 ID。
	TenantId string
	// OrgIds 表示当前主体关联的组织 ID 列表。
	OrgIds []string
	// PostIds 表示当前主体关联的岗位 ID 列表。
	PostIds []string
	// RoleIds 表示当前主体关联的角色 ID 列表。
	RoleIds []string
	// SubjectType 表示本次请求主体类型：anonymous/user/service。
	SubjectType string
	// InvokeAppId 表示本跳权限判定中的调用方应用 ID。
	InvokeAppId string
	// InvokeInstanceId 表示本跳调用方服务实例 ID，可为空。
	InvokeInstanceId string
	// TargetAppId 表示 authz 对 route.app_id 的判定语义。
	TargetAppId string
	// TargetInstanceId 表示被访问服务实例 ID，可为空。
	TargetInstanceId string
	// ApiMethod 表示本次授权动作，HTTP 为方法名，gRPC 为 GRPC。
	ApiMethod string
	// ApiPath 表示本次授权资源路径，HTTP 为入口 path，gRPC 为 FullMethod。
	ApiPath string
	// DecisionId 表示 authz allow 决策 ID。
	DecisionId string
	// AuthzSignJWS 保存 authz 注入的原始 compact JWS，来自 x-firefly-authz-sign metadata。
	AuthzSignJWS string
	// VerifiedAuthzSign 保存已本地验签通过的 JWS payload；未启用验签时为空。
	VerifiedAuthzSign *AuthzSign
	// TraceId 表示从当前 OTel span 提取的 trace 标识快照，不对应自定义 header。
	TraceId string
	// UserContext 保存用户身份上下文；启用验签时以 JWS payload 为准。
	UserContext *UserContext
	// InvokeServiceContext 保存本跳调用方服务身份上下文；启用验签时以 JWS payload 为准。
	InvokeServiceContext *InvokeServiceContext
	// TargetServiceContext 保存本跳被访问服务身份上下文；启用验签时以 JWS payload 为准。
	TargetServiceContext *TargetServiceContext
	// DecisionContext 保存 authz 决策事实；启用验签时以 JWS payload 为准。
	DecisionContext *DecisionContext
}

// UserContext 表示服务进程内可读取的用户身份上下文。
type UserContext struct {
	// UserId 是用户主体 ID。
	UserId string
	// AppId 是用户身份中的应用 ID。
	AppId string
	// TenantId 是用户所属租户 ID。
	TenantId string
	// Session 是 authz 从用户 access token 中解析出的会话标识。
	Session string
	// OrgIds 是用户关联组织 ID 列表。
	OrgIds []string
	// PostIds 是用户关联岗位 ID 列表。
	PostIds []string
	// RoleIds 是用户关联角色 ID 列表。
	RoleIds []string
}

// InvokeServiceContext 表示服务进程内可读取的当前跳调用方服务身份。
type InvokeServiceContext struct {
	// AppId 是当前这一跳调用方服务的应用 ID。
	AppId string
	// InstanceId 是当前这一跳调用方服务实例 ID，可为空。
	InstanceId string
}

// TargetServiceContext 表示服务进程内可读取的当前跳被访问服务身份。
type TargetServiceContext struct {
	// AppId 是 route 所属服务 app_id；authz 判定时解释为 target_app_id。
	AppId string
	// InstanceId 是 route 所属服务实例 ID，可为空。
	InstanceId string
}

// DecisionContext 表示服务进程内可读取的 authz 判定结果。
type DecisionContext struct {
	// SubjectType 表示本次请求主体类型：anonymous/user/service。
	SubjectType string
	// InvokeAppId 表示本跳权限判定中的调用方 app_id。
	InvokeAppId string
	// InvokeInstanceId 表示本跳调用方服务实例 ID，可为空。
	InvokeInstanceId string
	// TargetAppId 表示 authz 对 route.app_id 的判定语义。
	TargetAppId string
	// TargetInstanceId 表示本跳被访问服务实例 ID，可为空。
	TargetInstanceId string
	// ApiMethod 表示本次授权动作，HTTP 为方法名，gRPC 为 GRPC。
	ApiMethod string
	// ApiPath 表示本次授权资源路径，HTTP 为入口 path，gRPC 为 FullMethod。
	ApiPath string
	// DecisionId 表示 authz allow 决策 ID。
	DecisionId string
}

// BuildContextOptions 定义构建服务主上下文时的可选验签规则。
type BuildContextOptions struct {
	// AuthzVerification 配置后，BuildVerifiedContext 会用它校验 x-firefly-authz-sign JWS。
	AuthzVerification *AuthzSignVerificationOptions
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

	// 返回未验签版本；调用方若需要可信 payload，应改用 BuildVerifiedContext。
	return value
}

// BuildVerifiedContext 从入站 metadata 构造服务主上下文，并校验 authz compact JWS。
func BuildVerifiedContext(ctx context.Context, options BuildContextOptions) (*Context, error) {
	// 先按普通 metadata 构造上下文，保留未验签时的读取便利。
	value := BuildContext(ctx, options)
	// 未配置验签时只做普通 metadata 结构化，调用方应明确知道该上下文不是信任根。
	if options.AuthzVerification == nil {
		return value, nil
	}

	// 使用 authz 注入的 JWS 作为服务侧验签输入。
	authzSign, err := VerifyAuthzSign(value.AuthzSignJWS, *options.AuthzVerification)
	if err != nil {
		// 验签失败直接返回错误，由入口中间件转换成 gRPC 未认证错误。
		return nil, err
	}
	// 验签通过后，用可信 payload 覆盖普通 metadata，避免信任客户端伪造字段。
	value.applyVerifiedAuthzSign(authzSign)
	// 返回已经绑定可信 JWS payload 的进程内上下文。
	return value, nil
}

// buildContextFromMetadata 只处理 Firefly 标准 metadata 到服务主上下文的字段映射。
func buildContextFromMetadata(ctx context.Context, options BuildContextOptions) *Context {
	// 进程内上下文只从入站 metadata 和可选签名载荷构造。
	value := &Context{}

	// 只有 gRPC 入站 metadata 存在时才解析调用方上下文。
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// AppLanguage 是客户端偏好字段，不参与权限判断。
		value.AppLanguage = ParseMetaKey(md, constant.AppLanguage)
		// Session 来自 authz 对 token/session 的可信解析，不作为出站透传字段。
		value.Session = ParseMetaKey(md, constant.Session)
		// UserId 来自 authz allow 后注入的普通 metadata；服务主体通常为空。
		value.UserId = ParseMetaKey(md, constant.UserId)
		// AppId 只表达用户身份中的应用 ID，不再混用本跳 invoke_app_id。
		value.AppId = ParseMetaKey(md, constant.AppId)
		// InvokeAppId 表示本跳权限判定中的调用方应用 ID。
		value.InvokeAppId = ParseMetaKey(md, constant.InvokeAppId)
		// InvokeInstanceId 表示本跳调用方服务实例 ID，可能为空。
		value.InvokeInstanceId = ParseMetaKey(md, constant.InvokeInstanceId)
		// TenantId 表示主体租户，服务或公共接口可能为空或通配。
		value.TenantId = ParseMetaKey(md, constant.TenantId)
		// SubjectType 区分 anonymous/user/service。
		value.SubjectType = ParseMetaKey(md, constant.SubjectType)
		// TargetAppId 是 authz 对 route.app_id 的判定语义，不是 route 层字段名。
		value.TargetAppId = ParseMetaKey(md, constant.TargetAppId)
		// TargetInstanceId 表示被访问服务实例 ID，通常为空。
		value.TargetInstanceId = ParseMetaKey(md, constant.TargetInstanceId)
		// ApiMethod 是 authz 注入的授权动作读取便利，可信版本仍以 AuthzSign 为准。
		value.ApiMethod = ParseMetaKey(md, constant.ApiMethod)
		// ApiPath 是 authz 注入的授权路径读取便利，可信版本仍以 AuthzSign 为准。
		value.ApiPath = ParseMetaKey(md, constant.ApiPath)
		// DecisionId 用于把业务日志和 authz allow 决策关联起来。
		value.DecisionId = ParseMetaKey(md, constant.DecisionId)
		// AuthzSignJWS 保存原始 JWS，后续 BuildVerifiedContext 会用它验签。
		value.AuthzSignJWS = ParseMetaKey(md, constant.AuthzSign)
		// OrgIds 可能有多个 metadata value，必须复制为服务上下文独占切片。
		value.OrgIds = cloneStrings(md.Get(constant.OrgIds))
		// PostIds 同样复制，避免调用方后续修改 metadata 影响上下文。
		value.PostIds = cloneStrings(md.Get(constant.PostIds))
		// RoleIds 同样复制，避免调用方后续修改 metadata 影响上下文。
		value.RoleIds = cloneStrings(md.Get(constant.RoleIds))
	}

	// 普通 metadata 只提供读取便利，仍然按目标语义组装进程内分组，可信性由 JWS 验签决定。
	value.rebuildDerivedContexts()

	// 返回只完成结构化的上下文，是否可信由调用方选择是否验签决定。
	return value
}

// applyVerifiedAuthzSign 使用已验签 payload 覆盖普通 metadata，避免业务代码信任可伪造字段。
func (c *Context) applyVerifiedAuthzSign(authzSign *AuthzSign) {
	// 空 receiver 或空 claim 都直接返回，保持方法幂等和空安全。
	if c == nil || authzSign == nil {
		return
	}
	// 保存完整可信 claim，业务或日志需要追踪授权上下文时可读取。
	c.VerifiedAuthzSign = authzSign
	// UserId 以签名 claim 为准，防止客户端伪造普通 x-firefly-user-id。
	c.UserId = authzSign.UserId
	// AppId 以用户身份中的 app_id 为准，不能被 invoke_app_id 覆盖。
	c.AppId = authzSign.AppId
	// Session 以签名 claim 为准，便于业务侧按会话做只读关联。
	c.Session = authzSign.Session
	// TenantId 以签名 claim 为准。
	c.TenantId = authzSign.TenantId
	// SubjectType 以签名 claim 为准。
	c.SubjectType = authzSign.SubjectType
	// InvokeAppId 以签名 claim 为准。
	c.InvokeAppId = authzSign.InvokeAppId
	// InvokeInstanceId 以签名 claim 为准。
	c.InvokeInstanceId = authzSign.InvokeInstanceId
	// TargetAppId 以签名 claim 为准。
	c.TargetAppId = authzSign.TargetAppId
	// TargetInstanceId 以签名 claim 为准。
	c.TargetInstanceId = authzSign.TargetInstanceId
	// ApiMethod 以签名 claim 为准。
	c.ApiMethod = authzSign.ApiMethod
	// ApiPath 以签名 claim 为准。
	c.ApiPath = authzSign.ApiPath
	// DecisionId 以签名 claim 为准。
	c.DecisionId = authzSign.DecisionId
	// OrgIds 以签名 claim 为准。
	c.OrgIds = cloneStrings(authzSign.OrgIds)
	// PostIds 以签名 claim 为准。
	c.PostIds = cloneStrings(authzSign.PostIds)
	// RoleIds 以签名 claim 为准。
	c.RoleIds = cloneStrings(authzSign.RoleIds)
	// UserContext 直接使用签名里的结构化 claim，避免从平铺字段反推用户身份。
	if authzSign.UserContext != nil {
		c.UserContext = &UserContext{
			UserId:   authzSign.UserContext.UserId,
			AppId:    authzSign.UserContext.AppId,
			TenantId: authzSign.UserContext.TenantId,
			Session:  authzSign.UserContext.Session,
			OrgIds:   cloneStrings(authzSign.UserContext.OrgIds),
			PostIds:  cloneStrings(authzSign.UserContext.PostIds),
			RoleIds:  cloneStrings(authzSign.UserContext.RoleIds),
		}
	} else {
		c.UserContext = nil
	}
	// InvokeServiceContext 只来自结构化服务身份，不从 invoke_app_id 反推。
	if authzSign.InvokeServiceContext != nil {
		c.InvokeServiceContext = &InvokeServiceContext{
			AppId:      authzSign.InvokeServiceContext.AppId,
			InstanceId: authzSign.InvokeServiceContext.InstanceId,
		}
	} else {
		c.InvokeServiceContext = nil
	}
	// TargetServiceContext 只来自结构化 route 所属服务身份。
	if authzSign.TargetServiceContext != nil {
		c.TargetServiceContext = &TargetServiceContext{
			AppId:      authzSign.TargetServiceContext.AppId,
			InstanceId: authzSign.TargetServiceContext.InstanceId,
		}
	} else {
		c.TargetServiceContext = nil
	}
	// 决策上下文仍聚合本跳授权结果，便于日志读取。
	c.rebuildDecisionContext()
}

func (c *Context) rebuildDerivedContexts() {
	// 空 receiver 直接返回，便于调用方在 defer 或测试中安全使用。
	if c == nil {
		return
	}
	// 有用户身份字段时才构造 UserContext，避免服务/匿名请求误判为用户请求。
	if c.UserId != "" || c.AppId != "" || c.TenantId != "" || c.Session != "" || len(c.OrgIds) > 0 || len(c.PostIds) > 0 || len(c.RoleIds) > 0 {
		c.UserContext = &UserContext{
			UserId:   c.UserId,
			AppId:    c.AppId,
			TenantId: c.TenantId,
			Session:  c.Session,
			OrgIds:   cloneStrings(c.OrgIds),
			PostIds:  cloneStrings(c.PostIds),
			RoleIds:  cloneStrings(c.RoleIds),
		}
	} else {
		c.UserContext = nil
	}
	// InvokeServiceContext 只表达服务 token 解析出的当前跳调用服务身份。
	if c.SubjectType == constant.SubjectTypeService && (c.InvokeAppId != "" || c.InvokeInstanceId != "") {
		c.InvokeServiceContext = &InvokeServiceContext{
			AppId:      c.InvokeAppId,
			InstanceId: c.InvokeInstanceId,
		}
	} else {
		c.InvokeServiceContext = nil
	}
	// TargetServiceContext 只表达 route 映射出的被访问服务身份。
	if c.TargetAppId != "" || c.TargetInstanceId != "" {
		c.TargetServiceContext = &TargetServiceContext{
			AppId:      c.TargetAppId,
			InstanceId: c.TargetInstanceId,
		}
	} else {
		c.TargetServiceContext = nil
	}
	// 决策上下文只表达 authz 判定结果和本跳调用关系。
	c.rebuildDecisionContext()
}

func (c *Context) rebuildDecisionContext() {
	// 空 receiver 直接返回，便于调用方在 defer 或测试中安全使用。
	if c == nil {
		return
	}
	// 决策上下文只表达 authz 判定结果和本跳调用关系。
	if c.SubjectType != "" || c.InvokeAppId != "" || c.TargetAppId != "" || c.ApiMethod != "" || c.ApiPath != "" || c.DecisionId != "" {
		c.DecisionContext = &DecisionContext{
			SubjectType:      c.SubjectType,
			InvokeAppId:      c.InvokeAppId,
			InvokeInstanceId: c.InvokeInstanceId,
			TargetAppId:      c.TargetAppId,
			TargetInstanceId: c.TargetInstanceId,
			ApiMethod:        c.ApiMethod,
			ApiPath:          c.ApiPath,
			DecisionId:       c.DecisionId,
		}
	} else {
		c.DecisionContext = nil
	}
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
	// 返回第一个 metadata value，调用方负责决定是否 trim。
	return values[0]
}

func cloneStrings(values []string) []string {
	// 没有值时返回 nil，避免制造无意义空切片。
	if len(values) == 0 {
		return nil
	}
	// 复制切片，避免进程内 Context 共享 metadata 内部底层数组。
	return append([]string(nil), values...)
}
