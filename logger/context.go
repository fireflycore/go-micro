package logger

import (
	"context"

	"github.com/fireflycore/go-micro/service"
	"go.uber.org/zap"
)

// ContextFields 供访问日志显式提取请求归因，不自动附加到每条普通业务日志。
// 只投影已装配的业务视图，不读取原始 metadata，也不输出 session。
func ContextFields(ctx context.Context) []zap.Field {
	c, ok := service.FromContext(ctx)
	if !ok {
		return nil
	}

	values := map[string]string{
		"service_app_id":      c.ServiceAppId,
		"service_instance_id": c.ServiceInstanceId,
		"service_method":      c.ServiceMethod,
		"service_path":        c.ServicePath,
		"invoke_app_id":       c.InvokeAppId,
		"invoke_method":       c.InvokeMethod,
		"invoke_path":         c.InvokePath,
		"client_ip":           c.ClientIp,
		"app_language":        c.AppLanguage,
		"app_version":         c.AppVersion,
		"system_name":         c.SystemName,
		"system_version":      c.SystemVersion,
		"client_name":         c.ClientName,
		"client_version":      c.ClientVersion,
	}

	if c.UserContext != nil {
		values["user_id"] = c.UserContext.UserId
		values["app_id"] = c.UserContext.AppId
		values["tenant_id"] = c.UserContext.TenantId
	}

	if a := c.AuthzSignContext; a != nil {
		// 授权字段只读已验签投影，不采用普通 subject-type 等明文头。
		values["subject_type"] = a.SubjectType
		values["subject_app_id"] = a.SubjectAppId
		values["decision_id"] = a.DecisionId
		values["authz_method"] = a.Method
		values["authz_path"] = a.Path
	}

	fields := make([]zap.Field, 0, len(values)+2)
	for k, v := range values {
		// 无用户或匿名场景保留字段缺失，不用本地服务身份补齐用户字段。
		if v != "" {
			fields = append(fields, zap.String(k, v))
		}
	}

	if c.SystemType != 0 {
		fields = append(fields, zap.Int32("system_type", c.SystemType))
	}
	if c.ClientType != 0 {
		fields = append(fields, zap.Int32("client_type", c.ClientType))
	}

	return fields
}
