package service

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/fireflycore/go-micro/constant"
	"google.golang.org/grpc/metadata"
)

// ErrIncomingContextInvalid 表示允许项存在重复值、非法字符或超出长度限制。
var ErrIncomingContextInvalid = errors.New("incoming context is invalid")

// 客户端环境仅用于偏好和观测，允许收集不代表建立可信身份。
var clientEnvironmentHeaders = []string{
	constant.AppLanguage, constant.AppVersion, constant.SystemType, constant.SystemName,
	constant.SystemVersion, constant.ClientType, constant.ClientName, constant.ClientVersion,
}

// 普通 authz 头只保留为观测输入，业务身份仍必须来自签名。
var observedHeaders = []string{constant.UserId, constant.Session, constant.AppId, constant.TenantId, constant.SubjectType, constant.DecisionId}

// IncomingContext 收集入口原始值；它不是已认证身份，也不进入业务公开视图。
type IncomingContext struct {
	// ServiceMethod 是适配器实际捕获的方法，不从请求头或签名推断。
	ServiceMethod string
	// ServicePath 是实际 URL path 或 gRPC FullMethod。
	ServicePath string
	// UserAuthority 是待下一跳 authz 再验证的用户凭证，禁止记录原文。
	UserAuthority string `json:"-"`
	// AuthzSignJWS 是本跳本地验签输入，不向下一跳传播。
	AuthzSignJWS string `json:"-"`
	// InvokeMethod 是上一跳自报入口方法，仅供观测。
	InvokeMethod string
	// InvokePath 是上一跳自报入口路径，不作为权限或投递依据。
	InvokePath string
	// AppLanguage 是 Firefly 应用语言头，缺失时才使用 AcceptLanguage 归一值。
	AppLanguage string
	// AppVersion 是客户端应用版本观测值。
	AppVersion string
	// SystemType 是客户端系统类型枚举。
	SystemType int32
	// SystemName 是客户端系统名称。
	SystemName string
	// SystemVersion 是客户端系统版本。
	SystemVersion string
	// ClientType 是客户端类型枚举。
	ClientType int32
	// ClientName 是客户端名称。
	ClientName string
	// ClientVersion 是客户端版本。
	ClientVersion string
	// AcceptLanguage 仅作入站语言偏好备用输入，不通用透传。
	AcceptLanguage string
	// AuthzObservedHeaders 保存允许的六项明文观测头，不得据此建立身份。
	AuthzObservedHeaders map[string][]string `json:"-"`
	// ClientIp 来自连接对端或显式可信代理提取策略。
	ClientIp string
	// TraceParent 是入站 W3C 链路传播头。
	TraceParent string
	// TraceState 是入站厂商链路状态，不用于身份判断。
	TraceState string
	// Baggage 是入站标准遥测附加信息，不作为权限输入。
	Baggage string
}

// 两种默认格式化都隐藏原始输入，避免凭证经调试日志泄漏。
func (IncomingContext) String() string   { return "IncomingContext{redacted}" }
func (IncomingContext) GoString() string { return "IncomingContext{redacted}" }

// validObservation 按 UTF-8 字节数限长，并拒绝可能污染日志或协议的控制字符。
func validObservation(value string, limit int) bool {
	if len(value) > limit || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// CollectIncomingContext 按精确允许列表收集输入，不接收服务凭证或任意 Firefly 前缀头。
func CollectIncomingContext(md metadata.MD, method, path, clientIp string) (*IncomingContext, error) {
	allowed := append(append([]string{}, clientEnvironmentHeaders...), observedHeaders...)
	allowed = append(allowed, constant.UserAuthority, constant.AuthzSign, constant.InvokeMethod, constant.InvokePath,
		constant.AcceptLanguage, constant.TraceParent, constant.TraceState, constant.Baggage)

	// 上下文头均为标量；重复值直接拒绝，不采用首值或末值覆盖策略。
	for _, key := range allowed {
		v := md.Get(key)
		limit := 2048
		// 凭证、签名与 baggage 容量较大，但仍需独立限制总字节数。
		if key == constant.AuthzSign || key == constant.UserAuthority || key == constant.Baggage {
			limit = 16384
		}
		if key == constant.InvokeMethod {
			// 方法只需容纳协议动作，使用比路径更紧的上限。
			limit = 32
		}
		if len(v) > 1 || (len(v) == 1 && !validObservation(v[0], limit)) {
			return nil, ErrIncomingContextInvalid
		}
	}

	get := func(k string) string { return ParseMetaKey(md, k) }
	in := &IncomingContext{
		ServiceMethod: method, ServicePath: path, ClientIp: clientIp,
		UserAuthority: get(constant.UserAuthority), AuthzSignJWS: get(constant.AuthzSign),
		InvokeMethod: get(constant.InvokeMethod), InvokePath: get(constant.InvokePath),
		AppLanguage: get(constant.AppLanguage), AppVersion: get(constant.AppVersion),
		SystemType: parseContextInt(get(constant.SystemType)), SystemName: get(constant.SystemName), SystemVersion: get(constant.SystemVersion),
		ClientType: parseContextInt(get(constant.ClientType)), ClientName: get(constant.ClientName), ClientVersion: get(constant.ClientVersion),
		AcceptLanguage: get(constant.AcceptLanguage), TraceParent: get(constant.TraceParent), TraceState: get(constant.TraceState), Baggage: get(constant.Baggage),
		AuthzObservedHeaders: map[string][]string{},
	}
	if in.AppLanguage == "" {
		// 只提取首选语言，出站统一使用 Firefly 语言头。
		in.AppLanguage = strings.TrimSpace(strings.Split(strings.Split(in.AcceptLanguage, ",")[0], ";")[0])
	}

	for _, key := range observedHeaders {
		if v := md.Get(key); len(v) > 0 {
			in.AuthzObservedHeaders[key] = append([]string{}, v...)
		}
	}

	return in, nil
}

// PublicIncomingMetadata 复制不含凭证的入站允许项；明文身份项仍仅是观测。
func PublicIncomingMetadata(md metadata.MD) metadata.MD {
	out := metadata.MD{}
	keys := append(append([]string{}, clientEnvironmentHeaders...), observedHeaders...)
	keys = append(keys, constant.AcceptLanguage, constant.InvokeMethod, constant.InvokePath, constant.TraceParent, constant.TraceState, constant.Baggage)
	for _, key := range keys {
		if v := md.Get(key); len(v) > 0 {
			out[key] = append([]string{}, v...)
		}
	}
	return out
}
