package service

import (
	"context"
	"errors"
	"strconv"

	"github.com/fireflycore/go-micro/constant"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc/metadata"
)

// OutgoingTarget 描述本次客户端调用目标，不表示 authz 已允许该调用。
type OutgoingTarget struct {
	// TargetAppId 在客户端可以确定目标应用时填写，不生成普通身份头。
	TargetAppId string
	// TargetMethod 是本次目标 HTTP 动词或 GRPC。
	TargetMethod string
	// TargetPath 是本次目标 URL path 或 gRPC FullMethod。
	TargetPath string
}

// OutgoingContext 每次出站独立生成，不能覆盖入站或运行时对象。
type OutgoingContext struct {
	// OutgoingTarget 保存当次调用描述，与当前服务实际入口分离。
	OutgoingTarget
	// UserAuthority 只从框架私有路径继承，不从 UserContext 重新序列化。
	UserAuthority string `json:"-"`
	// ServiceAuthority 必须来自当前服务 provider，覆盖上一跳服务凭证。
	ServiceAuthority string `json:"-"`
	// InvokeAppId 是当前服务应用的出站归因值，不生成可自报的普通 AppId 头。
	InvokeAppId string
	// InvokeMethod 来自当前实际 ServiceMethod，不沿用收到的上一跳值。
	InvokeMethod string
	// InvokePath 来自当前实际 ServicePath，仅供下一跳观测。
	InvokePath string
	// AppLanguage 是已归一的应用语言偏好。
	AppLanguage string
	// AppVersion 是允许继承的客户端应用版本。
	AppVersion string
	// SystemType 是允许继承的客户端系统类型。
	SystemType int32
	// SystemName 是允许继承的客户端系统名称。
	SystemName string
	// SystemVersion 是允许继承的客户端系统版本。
	SystemVersion string
	// ClientType 是允许继承的客户端类型。
	ClientType int32
	// ClientName 是允许继承的客户端名称。
	ClientName string
	// ClientVersion 是允许继承的客户端版本。
	ClientVersion string
	// TraceParent 从当前活动出站 span 注入，不复制入口快照。
	TraceParent string
	// TraceState 由当前 OTel propagator 注入。
	TraceState string
	// Baggage 由当前 OTel propagator 注入，不包含额外身份授权语义。
	Baggage string
}

// 默认格式化不展开出站对象中的凭证。
func (OutgoingContext) String() string   { return "OutgoingContext{redacted}" }
func (OutgoingContext) GoString() string { return "OutgoingContext{redacted}" }

// BuildOutgoingContext 只读运行时视图，缺少当前服务凭证时失败，不降级为用户直调。
// ctx 必须非 nil，不以 Background 掩盖调用方丢失请求上下文的问题。
func BuildOutgoingContext(ctx context.Context, target OutgoingTarget, serviceAuthority string) (*OutgoingContext, error) {
	if serviceAuthority == "" {
		return nil, errors.New("service authority is required")
	}
	if !validObservation(serviceAuthority, 16384) || !validObservation(target.TargetMethod, 32) || !validObservation(target.TargetPath, 2048) {
		return nil, ErrIncomingContextInvalid
	}

	c, _ := FromContext(ctx)
	if c == nil {
		// 后台任务可以没有业务入口，但仍必须提供当前服务凭证。
		c = &Context{}
	}

	if !validObservation(c.ServiceMethod, 32) || !validObservation(c.ServicePath, 2048) {
		return nil, ErrIncomingContextInvalid
	}

	for _, value := range []string{c.AppLanguage, c.AppVersion, c.SystemName, c.SystemVersion, c.ClientName, c.ClientVersion} {
		if !validObservation(value, 2048) {
			return nil, ErrIncomingContextInvalid
		}
	}

	user, _ := ctx.Value(authorityValueKey).(string)
	// 创建新对象，确保顺序和并发调用之间不会共享可修改的出站字段。
	out := &OutgoingContext{
		OutgoingTarget: target, UserAuthority: user, ServiceAuthority: serviceAuthority,
		InvokeAppId: c.ServiceAppId, InvokeMethod: c.ServiceMethod, InvokePath: c.ServicePath,
		AppLanguage: c.AppLanguage, AppVersion: c.AppVersion, SystemType: c.SystemType, SystemName: c.SystemName, SystemVersion: c.SystemVersion,
		ClientType: c.ClientType, ClientName: c.ClientName, ClientVersion: c.ClientVersion,
	}

	carrier := propagation.MapCarrier{}
	p := otel.GetTextMapPropagator()
	if len(p.Fields()) == 0 {
		// 未全局配置传播器时仍保持标准 W3C 传播能力。
		p = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	}

	p.Inject(ctx, carrier)
	out.TraceParent, out.TraceState, out.Baggage = carrier.Get(constant.TraceParent), carrier.Get(constant.TraceState), carrier.Get(constant.Baggage)

	return out, nil
}

// Metadata 仅编码精确出站白名单，不含上一跳签名、明文身份或组件/IP 头。
func (o *OutgoingContext) Metadata() metadata.MD {
	md := metadata.MD{}
	values := map[string]string{
		constant.UserAuthority: o.UserAuthority, constant.ServiceAuthority: o.ServiceAuthority,
		constant.InvokeMethod: o.InvokeMethod, constant.InvokePath: o.InvokePath,
		constant.AppLanguage: o.AppLanguage, constant.AppVersion: o.AppVersion,
		constant.SystemName: o.SystemName, constant.SystemVersion: o.SystemVersion,
		constant.ClientName: o.ClientName, constant.ClientVersion: o.ClientVersion,
		constant.TraceParent: o.TraceParent, constant.TraceState: o.TraceState, constant.Baggage: o.Baggage,
	}

	if o.SystemType != 0 {
		values[constant.SystemType] = strconv.FormatInt(int64(o.SystemType), 10)
	}
	if o.ClientType != 0 {
		values[constant.ClientType] = strconv.FormatInt(int64(o.ClientType), 10)
	}

	for k, v := range values {
		// 空值不发送，避免生成没有实际内容的上下文头。
		if v != "" {
			md.Set(k, v)
		}
	}

	return md
}
