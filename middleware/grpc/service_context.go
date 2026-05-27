package gm

import (
	"context"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ServiceContextInterceptorOptions 定义拦截器选项。
type ServiceContextInterceptorOptions struct {
	// ServiceAppId 表示当前服务自身 app_id，用于注入 ServiceContext 并默认校验 target_app_id。
	ServiceAppId string
	// ServiceInstanceId 表示当前服务实例 ID，用于日志和实例级排障。
	ServiceInstanceId string
	// AuthzVerification 非空时，入口会本地验签 x-firefly-authz-context。
	AuthzVerification *service.AuthzContextVerificationOptions
	// AuthzSkipMethods 表示不执行 authz 上下文验签的 gRPC 完整方法名，常用于 health check。
	AuthzSkipMethods []string
}

// NewServiceContextUnaryInterceptor 在请求入口统一建立并注入 service.Context。
func NewServiceContextUnaryInterceptor(options ServiceContextInterceptorOptions) grpc.UnaryServerInterceptor {
	// 预先构造基础 BuildContextOptions，避免每次请求重复复制服务自身身份。
	buildOptions := service.BuildContextOptions{
		ServiceAppId:      options.ServiceAppId,
		ServiceInstanceId: options.ServiceInstanceId,
	}
	// 预先整理跳过验签的方法集合，热路径只做 map 查询。
	skipAuthzMethods := buildServiceContextAuthzSkipMethods(options.AuthzSkipMethods)

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// 根据配置决定只结构化上下文，还是结构化后再校验 authz JWS。
		serviceContext, err := buildServiceContext(ctx, info, buildOptions, options.AuthzVerification, skipAuthzMethods)
		if err != nil {
			// 验签失败说明入口身份不可被信任，统一返回 Unauthenticated。
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		// 构建成功后把 ServiceContext 注入 ctx，业务层统一从 service.FromContext 读取。
		if serviceContext != nil {
			ctx = service.WithContext(ctx, serviceContext)
		}

		// 继续执行后续拦截器或业务 handler。
		return handler(ctx, req)
	}
}

func buildServiceContext(ctx context.Context, info *grpc.UnaryServerInfo, buildOptions service.BuildContextOptions, verification *service.AuthzContextVerificationOptions, skipAuthzMethods map[string]struct{}) (*service.Context, error) {
	// 没有配置验签或当前方法明确跳过验签时，只构造普通 ServiceContext。
	if verification == nil || shouldSkipServiceContextAuthz(info, skipAuthzMethods) {
		return service.BuildContext(ctx, buildOptions), nil
	}

	// 每次请求复制验签选项，避免把当前 gRPC 方法推导出的期望值写回共享配置。
	resolvedVerification := *verification
	// 未显式配置 target_app_id 时，默认要求请求目标就是当前服务 app_id。
	if resolvedVerification.ExpectedTargetAppId == "" {
		resolvedVerification.ExpectedTargetAppId = buildOptions.ServiceAppId
	}
	// gRPC 服务端入口的授权动作固定为 GRPC。
	if resolvedVerification.ExpectedResourceType == "" {
		resolvedVerification.ExpectedResourceType = constant.RequestMethodGrpcString
	}
	// 未显式配置资源路径时，使用当前 gRPC FullMethod 校验授权结果不可跨方法复用。
	if resolvedVerification.ExpectedResourcePath == "" && info != nil {
		resolvedVerification.ExpectedResourcePath = info.FullMethod
	}

	// 把本次请求解析出的期望值放回 buildOptions，交给 service 层完成实际验签。
	buildOptions.AuthzVerification = &resolvedVerification
	// 返回已验签的 ServiceContext；失败时返回明确错误。
	return service.BuildVerifiedContext(ctx, buildOptions)
}

func buildServiceContextAuthzSkipMethods(methods []string) map[string]struct{} {
	// 没有跳过规则时返回 nil，后续判断可以直接走快速分支。
	if len(methods) == 0 {
		return nil
	}
	// 使用 map 表达集合，避免每次请求线性扫描。
	result := make(map[string]struct{}, len(methods))
	// 逐个整理调用方传入的完整 gRPC method。
	for _, method := range methods {
		// 空字符串不是有效 method，直接忽略。
		if method == "" {
			continue
		}
		// struct{} 不占额外值空间，适合作为 set value。
		result[method] = struct{}{}
	}
	// 返回只读使用的跳过集合。
	return result
}

func shouldSkipServiceContextAuthz(info *grpc.UnaryServerInfo, methods map[string]struct{}) bool {
	// 没有配置跳过集合或缺少 gRPC 方法信息时，不跳过验签。
	if len(methods) == 0 || info == nil {
		return false
	}
	// FullMethod 命中集合时跳过验签，常见于 health check。
	_, ok := methods[info.FullMethod]
	// 返回是否跳过当前方法。
	return ok
}
