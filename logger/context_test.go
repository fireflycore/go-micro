package logger

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/internal/contexttest"
	"github.com/fireflycore/go-micro/service"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc/metadata"
)

// 验证日志只信任验签视图，ORM 投影维持五项身份，访问日志保留必要入口归因。
func TestContextLogsTrustedIdentityWithoutSecrets(t *testing.T) {
	o := contexttest.Options()
	ctx, err := service.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.AuthzSign, contexttest.Sign(contexttest.Claims("service", true)),
		constant.UserAuthority, "secret-token", constant.UserId, "forged-user", constant.Session, "secret-session",
	), service.BuildContextOptions{ServiceAppId: "B", ServiceMethod: "GRPC", ServicePath: "/items.Service/Get", AuthzVerification: &o})
	if err != nil {
		t.Fatal(err)
	}
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(NewContextOmittingCore(core))
	// 普通业务日志只补充既有链路字段，不自动复制身份、入口、客户端环境或授权投影。
	log.Info("service", appendContextFields(ctx, "service", []zap.Field{zap.String("operation", "test")})...)
	serviceFields := logs.All()[0].ContextMap()
	if len(serviceFields) != 2 || serviceFields["operation"] != "test" || serviceFields["log_type"] != "service" {
		t.Fatalf("ordinary service log expanded request context: %v", serviceFields)
	}
	// 访问日志显式选取请求投影；与普通业务日志和 ORM 身份投影分开。
	log.Info("access", appendContextFields(ctx, "access", ContextFields(ctx))...)
	fields := logs.All()[1].ContextMap()
	if fields["user_id"] != "user" || fields["service_app_id"] != "B" {
		t.Fatal(fields)
	}
	identity := service.TrustedLogIdentity(ctx)
	if identity != (service.LogIdentity{UserId: "user", AppId: "login-app", TenantId: "user-tenant", ServiceAppId: "B"}) {
		t.Fatal(identity)
	}
	identityJSON, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	var identityFields map[string]any
	if err := json.Unmarshal(identityJSON, &identityFields); err != nil {
		t.Fatal(err)
	}
	if len(identityFields) != 5 {
		t.Fatalf("ORM projection must contain only five identity fields: %v", identityFields)
	}
	if fields["service_path"] != "/items.Service/Get" || fields["invoke_app_id"] != "A" {
		t.Fatalf("access context lost request attribution: %v", fields)
	}
	b, _ := json.Marshal(fields)
	// 检查最终可序列化字段，不能让 token、session 或隐藏 ctx 绕过默认日志边界。
	if strings.Contains(string(b), "secret") || strings.Contains(string(b), "forged") || fields["ctx"] != nil {
		t.Fatal("sensitive default log")
	}
}
