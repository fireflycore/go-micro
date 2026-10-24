package service

import (
	"context"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

type contextKey string

// 使用独立键类型隔离框架内部值，避免与业务字符串键冲突。
const serviceContextValueKey contextKey = "service.context"

// 用户凭证单独保存在私有键中，不进入公开的业务上下文。
const authorityValueKey contextKey = "service.authority"

// UserContext 是验签后保留的用户身份；服务主体调用也可以独立携带此视图。
type UserContext struct {
	// Session 是用户会话标识，不是原始 token，默认日志仍不得输出它。
	Session string `json:"session"`
	// UserId 是已验证的用户标识。
	UserId string `json:"user_id"`
	// AppId 是用户登录应用，不表示本服务或上一跳服务。
	AppId string `json:"app_id"`
	// TenantId 是用户租户，不要求与服务主体的租户相同。
	TenantId string `json:"tenant_id"`
}

// 格式化输出不展开用户身份，防止 session 被默认日志带出。
func (UserContext) String() string   { return "UserContext{session:redacted}" }
func (UserContext) GoString() string { return "UserContext{session:redacted}" }

// TraceContext 仅用于入口日志快照，不替代 context.Context 中的活动 span。
type TraceContext struct {
	// TraceId 是构建入口上下文时的链路标识。
	TraceId string
	// SpanId 是构建入口上下文时的 span 标识。
	SpanId string
	// ParentSpanId 供入口观测使用；未采集时保持为空，不推断父 span。
	ParentSpanId string
}

// Context 是每个请求的业务视图，不保存原始凭证、JWS 或传输 metadata。
type Context struct {
	// ServiceAppId 来自本地配置，表示当前服务应用。
	ServiceAppId string
	// ServiceInstanceId 来自本地配置，表示当前服务实例。
	ServiceInstanceId string
	// ServiceMethod 来自实际适配器入口，HTTP 为动词，gRPC 固定为 GRPC。
	ServiceMethod string
	// ServicePath 来自实际 URL path 或 gRPC FullMethod，不能由签名反填。
	ServicePath string
	// UserContext 只来自验签结果；纯服务或匿名调用可以没有用户。
	UserContext *UserContext
	// AuthzSignContext 是已验签的授权投影，不包含签名串。
	AuthzSignContext *AuthzSignContext
	// InvokeAppId 只派生自已验签的 invoke_service_app_id，不回退到用户应用。
	InvokeAppId string
	// InvokeMethod 是上一跳服务上报的方法，仅用于观测，不参与授权。
	InvokeMethod string
	// InvokePath 是上一跳服务上报的入口，仅用于观测，不参与绑定校验。
	InvokePath string
	// AppLanguage 是归一后的应用语言偏好，不是可信身份。
	AppLanguage string
	// AppVersion 是客户端上报的应用版本。
	AppVersion string
	// SystemType 是客户端上报的系统类型枚举，不参与身份判断。
	SystemType int32
	// SystemName 是客户端上报的系统名称。
	SystemName string
	// SystemVersion 是客户端上报的系统版本。
	SystemVersion string
	// ClientType 是客户端上报的客户端类型枚举，不参与身份判断。
	ClientType int32
	// ClientName 是客户端上报的客户端名称。
	ClientName string
	// ClientVersion 是客户端上报的客户端版本。
	ClientVersion string
	// ClientIp 由入口受控提取，只用于本地观测，不通用透传。
	ClientIp string
	// TraceContext 是入口快照；每次出站仍从当前活动 span 注入遥测。
	TraceContext *TraceContext
}

// BuildContextOptions 由本地装配与实际协议适配器提供，不从普通身份头填充。
type BuildContextOptions struct {
	// ServiceAppId 是需要与签名投递绑定比较的本地服务应用。
	ServiceAppId string
	// ServiceInstanceId 是当前实例，仅用于本地上下文。
	ServiceInstanceId string
	// ServiceMethod 是适配器捕获的实际入口方法。
	ServiceMethod string
	// ServicePath 是适配器捕获的实际入口路径。
	ServicePath string
	// ClientIp 默认取连接对端，可信代理策略应在适配器中显式处理。
	ClientIp string
	// AuthzVerification 非空时必须通过本地验签；为空不会建立可信用户身份。
	AuthzVerification *AuthzSignVerificationOptions
}

// cloneContext 复制嵌套投影，避免读取者或出站调用修改请求级原对象。
func cloneContext(c *Context) *Context {
	if c == nil {
		return nil
	}
	value := *c
	if c.UserContext != nil {
		u := *c.UserContext
		value.UserContext = &u
	}
	if c.AuthzSignContext != nil {
		a := *c.AuthzSignContext
		value.AuthzSignContext = &a
	}
	if c.TraceContext != nil {
		t := *c.TraceContext
		value.TraceContext = &t
	}
	return &value
}

// WithContext 保存业务视图的副本；取消、期限及活动 span 仍由原生 ctx 承担。
func WithContext(ctx context.Context, value *Context) context.Context {
	if ctx == nil || value == nil {
		return ctx
	}
	return context.WithValue(ctx, serviceContextValueKey, cloneContext(value))
}

// FromContext 返回副本，不允许通过修改返回值污染同请求的其他调用。
func FromContext(ctx context.Context) (*Context, bool) {
	if ctx == nil {
		return nil, false
	}
	value, ok := ctx.Value(serviceContextValueKey).(*Context)
	return cloneContext(value), ok
}

// MustFromContext 适用于明确要求已装配业务入口的路径，缺失时直接暴露装配错误。
func MustFromContext(ctx context.Context) *Context {
	value, ok := FromContext(ctx)
	if !ok {
		panic("service: context not found in context")
	}
	return value
}

// BuildContext 不从普通 metadata 建立身份，可信身份只能来自本地验签。
func BuildContext(ctx context.Context, options BuildContextOptions) (*Context, error) {
	return BuildVerifiedContext(ctx, options)
}

// BuildVerifiedContext 只构造公开视图；需要私有凭证继承时由入口调用 NewIncomingContext。
func BuildVerifiedContext(ctx context.Context, options BuildContextOptions) (*Context, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	in, err := CollectIncomingContext(md, options.ServiceMethod, options.ServicePath, options.ClientIp)
	if err != nil {
		return nil, err
	}
	return buildIncomingContext(ctx, in, options)
}

// buildIncomingContext 先构建本地事实，再以验签结果补充用户和授权投影。
func buildIncomingContext(ctx context.Context, in *IncomingContext, options BuildContextOptions) (*Context, error) {
	value := contextFromIncoming(ctx, in, options)
	if options.AuthzVerification == nil {
		return value, nil
	}

	o := *options.AuthzVerification
	// 绑定比较必须使用本地服务与实际入口，覆盖选项中可能残留的预设值。
	o.ExpectedServiceAppId, o.ExpectedServiceMethod, o.ExpectedServicePath = options.ServiceAppId, in.ServiceMethod, in.ServicePath
	sign, err := VerifyAuthzSign(in.AuthzSignJWS, o)
	if err != nil {
		return nil, err
	}

	value.AuthzSignContext = &sign.AuthzSignContext
	if sign.UserContext != nil {
		u := *sign.UserContext
		value.UserContext = &u
	}

	value.InvokeAppId = sign.InvokeServiceAppId
	// 无已认证上一跳服务时，不保留客户端伪造的 Invoke 入口记录。
	if value.InvokeAppId != "" {
		value.InvokeMethod, value.InvokePath = in.InvokeMethod, in.InvokePath
	}

	return value, nil
}

// contextFromIncoming 只映射本地事实与观测值，不提升明文身份头的可信度。
func contextFromIncoming(ctx context.Context, in *IncomingContext, o BuildContextOptions) *Context {
	value := &Context{
		ServiceAppId: o.ServiceAppId, ServiceInstanceId: o.ServiceInstanceId, ServiceMethod: in.ServiceMethod, ServicePath: in.ServicePath,
		AppLanguage: in.AppLanguage, AppVersion: in.AppVersion, SystemType: in.SystemType, SystemName: in.SystemName, SystemVersion: in.SystemVersion,
		ClientType: in.ClientType, ClientName: in.ClientName, ClientVersion: in.ClientVersion, ClientIp: in.ClientIp,
	}

	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		value.TraceContext = &TraceContext{TraceId: sc.TraceID().String(), SpanId: sc.SpanID().String()}
	}

	return value
}

// NewIncomingContext 在清理传输输入前私有保存用户凭证，再注入公开业务视图。
// 服务凭证只由 authz 消费，上一跳签名只用于本地验签，二者都不公开给业务。
func NewIncomingContext(ctx context.Context, md metadata.MD, options BuildContextOptions) (context.Context, error) {
	in, err := CollectIncomingContext(md, options.ServiceMethod, options.ServicePath, options.ClientIp)
	if err != nil {
		return nil, err
	}

	if !trace.SpanContextFromContext(ctx).IsValid() {
		// 已有活动 span 时不以入站头覆盖它；没有时才提取标准遥测头。
		p := otel.GetTextMapPropagator()
		if len(p.Fields()) == 0 {
			p = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
		}

		carrier := propagation.MapCarrier{}
		for _, key := range []string{"traceparent", "tracestate", "baggage"} {
			carrier[key] = ParseMetaKey(md, key)
		}
		ctx = p.Extract(ctx, carrier)
	}

	value, err := buildIncomingContext(ctx, in, options)
	if err != nil {
		return nil, err
	}

	ctx = metadata.NewIncomingContext(ctx, PublicIncomingMetadata(md))
	// 不把用户凭证混入 metadata 或 Context 字段，只允许框架出站路径继承。
	ctx = context.WithValue(ctx, authorityValueKey, in.UserAuthority)

	return WithContext(ctx, value), nil
}

// ParseMetaKey 读取标量首值；允许列表中的重复值应已由收集阶段拒绝。
func ParseMetaKey(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// parseContextInt 按 int32 解析观测枚举，其结果不用于身份或授权判断。
func parseContextInt(raw string) int32 {
	value, _ := strconv.ParseInt(raw, 10, 32)
	return int32(value)
}

// LogIdentity 是供 ORM 适配的最小身份投影，不复制完整请求或 Invoke 入口。
type LogIdentity struct {
	// UserId 是验签得到的用户标识，无用户时为空。
	UserId string
	// AppId 是用户登录应用，不是服务调用主体。
	AppId string
	// TenantId 是用户租户。
	TenantId string
	// ServiceAppId 是本地服务应用。
	ServiceAppId string
	// ServiceInstanceId 是本地服务实例。
	ServiceInstanceId string
}

// TrustedLogIdentity 不读取原始 metadata，也不输出 session 或凭证。
func TrustedLogIdentity(ctx context.Context) LogIdentity {
	c, ok := FromContext(ctx)
	if !ok {
		return LogIdentity{}
	}

	i := LogIdentity{
		ServiceAppId: c.ServiceAppId, ServiceInstanceId: c.ServiceInstanceId,
	}

	if c.UserContext != nil {
		i.UserId, i.AppId, i.TenantId = c.UserContext.UserId, c.UserContext.AppId, c.UserContext.TenantId
	}

	return i
}
