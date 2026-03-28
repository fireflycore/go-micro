package invocation

import (
	"context"
	"time"

	"github.com/fireflycore/go-micro/constant"
	"google.golang.org/grpc/metadata"
)

// Caller 表示一次服务调用的发起方身份。
//
// 这里聚焦的是调用治理与 Authz 所需的统一字段，
// 避免让每个服务都手工拼装一套不同的 metadata。
type Caller struct {
	// UserId 表示当前用户身份，可为空。
	UserId string `json:"user_id"`
	// AppId 表示当前应用身份，可为空。
	AppId string `json:"app_id"`
	// TenantId 表示当前租户身份，可为空。
	TenantId string `json:"tenant_id"`
	// OrgIds 表示当前组织范围，可为空。
	OrgIds []string `json:"org_ids"`
	// RoleIds 表示当前角色范围，可为空。
	RoleIds []string `json:"role_ids"`
}

// InvocationContext 表示一次调用附带的统一上下文。
//
// 它的职责不是替代 context.Context，
// 而是把“应当被稳定传递和审计的调用元信息”沉淀为一个结构化对象。
type InvocationContext struct {
	// TraceId 表示当前调用使用的链路 ID。
	TraceId string `json:"trace_id"`
	// Caller 表示调用者身份。
	Caller Caller `json:"caller"`
	// Metadata 表示调用方额外附带的 metadata。
	// 这里允许业务侧做补充，但核心库仍会统一注入标准字段。
	Metadata metadata.MD `json:"-"`
	// Timeout 表示本次调用的超时时间。
	Timeout time.Duration `json:"timeout"`
}

// Clone 返回 InvocationContext 的深拷贝。
// 该方法用于避免调用方复用同一个 metadata 引起数据串扰。
func (c InvocationContext) Clone() InvocationContext {
	if c.Metadata != nil {
		c.Metadata = c.Metadata.Copy()
	}
	c.Caller.OrgIds = append([]string(nil), c.Caller.OrgIds...)
	c.Caller.RoleIds = append([]string(nil), c.Caller.RoleIds...)
	return c
}

// BuildMetadata 将 InvocationContext 转换为标准 gRPC metadata。
//
// 约定：
// - 调用方显式提供的 Metadata 会被保留；
// - 标准字段由核心库统一写入，避免上层重复设置；
// - 对于切片字段，会整体覆盖为当前上下文中的值。
func (c InvocationContext) BuildMetadata() metadata.MD {
	md := metadata.MD{}
	if c.Metadata != nil {
		md = c.Metadata.Copy()
	}

	if c.TraceId != "" {
		md.Set(constant.TraceId, c.TraceId)
	}
	if c.Caller.UserId != "" {
		md.Set(constant.UserId, c.Caller.UserId)
	}
	if c.Caller.AppId != "" {
		md.Set(constant.AppId, c.Caller.AppId)
	}
	if c.Caller.TenantId != "" {
		md.Set(constant.TenantId, c.Caller.TenantId)
	}
	if len(c.Caller.OrgIds) != 0 {
		md.Set(constant.OrgIds, c.Caller.OrgIds...)
	}
	if len(c.Caller.RoleIds) != 0 {
		md.Set(constant.RoleIds, c.Caller.RoleIds...)
	}

	return md
}

// NewOutgoingContext 基于父上下文构造新的 gRPC 出站上下文。
//
// 行为说明：
// - 若 parent 为 nil，则使用 Background；
// - metadata 会统一写入 outgoing context；
// - 若 Timeout > 0，则自动附加超时控制。
func (c InvocationContext) NewOutgoingContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}

	md := c.BuildMetadata()
	ctx := metadata.NewOutgoingContext(context.WithoutCancel(parent), md)
	if c.Timeout > 0 {
		return context.WithTimeout(ctx, c.Timeout)
	}

	return ctx, func() {}
}

// AuthzContext 表示外挂 Authz 所需的标准化输入。
//
// 该对象不直接绑定某个 Authz 实现，
// 只表达“做权限判断时必须稳定得到的字段”。
type AuthzContext struct {
	// Service 表示目标服务身份。
	Service ServiceRef `json:"service"`
	// FullMethod 表示完整 gRPC 方法，例如 /acme.user.v1.UserService/GetUser。
	FullMethod string `json:"full_method"`
	// TraceID 表示链路 ID。
	TraceId string `json:"trace_id"`
	// Caller 表示调用方身份。
	Caller Caller `json:"caller"`
	// Metadata 表示构造判定时附带的完整 metadata 副本。
	Metadata metadata.MD `json:"-"`
}

// NewAuthzContext 根据 ServiceRef、方法名和 InvocationContext 生成标准 AuthzContext。
func NewAuthzContext(ref ServiceRef, method string, invocation InvocationContext) AuthzContext {
	invocation = invocation.Clone()
	return AuthzContext{
		Service:    ref,
		FullMethod: method,
		TraceId:    invocation.TraceId,
		Caller:     invocation.Caller,
		Metadata:   invocation.BuildMetadata(),
	}
}
