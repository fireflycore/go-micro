package invocation

import (
	"context"
	"testing"

	"github.com/fireflycore/go-micro/constant"
	"google.golang.org/grpc/metadata"
)

func TestWithUserContext_And_UserContextFromContext(t *testing.T) {
	tests := []struct {
		name     string
		meta     *UserContextMeta
		wantOk   bool
		wantMeta *UserContextMeta
	}{
		{
			name: "normal case",
			meta: &UserContextMeta{
				Session:  "session-123",
				ClientIp: "192.168.1.1",
				UserId:   "user-1",
				AppId:    "app-1",
				TenantId: "tenant-1",
				RoleIds:  []string{"role-1", "role-2"},
				OrgIds:   []string{"org-1"},
			},
			wantOk: true,
			wantMeta: &UserContextMeta{
				Session:  "session-123",
				ClientIp: "192.168.1.1",
				UserId:   "user-1",
				AppId:    "app-1",
				TenantId: "tenant-1",
				RoleIds:  []string{"role-1", "role-2"},
				OrgIds:   []string{"org-1"},
			},
		},
		{
			name:     "nil meta",
			meta:     nil,
			wantOk:   false,
			wantMeta: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// 存入 context
			ctx = WithUserContext(ctx, tt.meta)

			// 从 context 获取
			gotMeta, gotOk := UserContextFromContext(ctx)

			if gotOk != tt.wantOk {
				t.Errorf("UserContextFromContext() ok = %v, want %v", gotOk, tt.wantOk)
			}

			if tt.wantOk {
				if gotMeta == nil {
					t.Fatal("UserContextFromContext() returned nil meta")
				}
				if gotMeta.Session != tt.wantMeta.Session {
					t.Errorf("Session = %v, want %v", gotMeta.Session, tt.wantMeta.Session)
				}
				if gotMeta.UserId != tt.wantMeta.UserId {
					t.Errorf("UserId = %v, want %v", gotMeta.UserId, tt.wantMeta.UserId)
				}
				if gotMeta.TenantId != tt.wantMeta.TenantId {
					t.Errorf("TenantId = %v, want %v", gotMeta.TenantId, tt.wantMeta.TenantId)
				}
			}
		})
	}
}

func TestUserContextFromContext_NilContext(t *testing.T) {
	meta, ok := UserContextFromContext(nil)
	if ok {
		t.Error("UserContextFromContext(nil) should return ok=false")
	}
	if meta != nil {
		t.Error("UserContextFromContext(nil) should return nil meta")
	}
}

func TestUserContextFromContext_EmptyContext(t *testing.T) {
	ctx := context.Background()
	meta, ok := UserContextFromContext(ctx)
	if ok {
		t.Error("UserContextFromContext(empty ctx) should return ok=false")
	}
	if meta != nil {
		t.Error("UserContextFromContext(empty ctx) should return nil meta")
	}
}

func TestMustUserContextFromContext_Success(t *testing.T) {
	ctx := context.Background()
	wantMeta := &UserContextMeta{
		UserId:   "user-1",
		TenantId: "tenant-1",
	}

	ctx = WithUserContext(ctx, wantMeta)
	gotMeta := MustUserContextFromContext(ctx)

	if gotMeta.UserId != wantMeta.UserId {
		t.Errorf("UserId = %v, want %v", gotMeta.UserId, wantMeta.UserId)
	}
}

func TestMustUserContextFromContext_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustUserContextFromContext should panic when context is empty")
		}
	}()

	ctx := context.Background()
	_ = MustUserContextFromContext(ctx)
}

func TestParseUserContextMeta_Success(t *testing.T) {
	md := metadata.MD{
		constant.Session:  []string{"session-123"},
		constant.ClientIp: []string{"192.168.1.1"},
		constant.UserId:   []string{"user-1"},
		constant.AppId:    []string{"app-1"},
		constant.TenantId: []string{"tenant-1"},
		constant.RoleIds:  []string{"role-1", "role-2"},
		constant.OrgIds:   []string{"org-1"},
	}

	meta, err := ParseUserContextMeta(md)
	if err != nil {
		t.Fatalf("ParseUserContextMeta() error = %v", err)
	}

	if meta.Session != "session-123" {
		t.Errorf("Session = %v, want session-123", meta.Session)
	}
	if meta.UserId != "user-1" {
		t.Errorf("UserId = %v, want user-1", meta.UserId)
	}
	if meta.TenantId != "tenant-1" {
		t.Errorf("TenantId = %v, want tenant-1", meta.TenantId)
	}
	if len(meta.RoleIds) != 2 {
		t.Errorf("len(RoleIds) = %v, want 2", len(meta.RoleIds))
	}
}

func TestParseUserContextMeta_MissingRequiredField(t *testing.T) {
	tests := []struct {
		name string
		md   metadata.MD
	}{
		{
			name: "missing session",
			md: metadata.MD{
				constant.ClientIp: []string{"192.168.1.1"},
				constant.UserId:   []string{"user-1"},
				constant.AppId:    []string{"app-1"},
				constant.TenantId: []string{"tenant-1"},
			},
		},
		{
			name: "missing user_id",
			md: metadata.MD{
				constant.Session:  []string{"session-123"},
				constant.ClientIp: []string{"192.168.1.1"},
				constant.AppId:    []string{"app-1"},
				constant.TenantId: []string{"tenant-1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseUserContextMeta(tt.md)
			if err == nil {
				t.Error("ParseUserContextMeta() should return error for missing required field")
			}
		})
	}
}

// BenchmarkParseUserContextMeta 测试解析性能
func BenchmarkParseUserContextMeta(b *testing.B) {
	md := metadata.MD{
		constant.Session:  []string{"session-123"},
		constant.ClientIp: []string{"192.168.1.1"},
		constant.UserId:   []string{"user-1"},
		constant.AppId:    []string{"app-1"},
		constant.TenantId: []string{"tenant-1"},
		constant.RoleIds:  []string{"role-1", "role-2"},
		constant.OrgIds:   []string{"org-1"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseUserContextMeta(md)
	}
}

// BenchmarkUserContextFromContext 测试从 context 获取的性能
func BenchmarkUserContextFromContext(b *testing.B) {
	ctx := context.Background()
	meta := &UserContextMeta{
		UserId:   "user-1",
		TenantId: "tenant-1",
	}
	ctx = WithUserContext(ctx, meta)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = UserContextFromContext(ctx)
	}
}

func TestNewAuthzContext_NilInvocationContext(t *testing.T) {
	service := &ServiceDNS{
		Service:   "auth",
		Namespace: "default",
	}

	authz := NewAuthzContext(service, "/acme.auth.app.v1.AuthAppService/GetAppSecret", nil)
	if authz == nil {
		t.Fatal("expected authz context, got nil")
	}
	if authz.Service != service {
		t.Fatalf("unexpected service pointer: %+v", authz.Service)
	}
	if authz.FullMethod != "/acme.auth.app.v1.AuthAppService/GetAppSecret" {
		t.Fatalf("unexpected full method: %s", authz.FullMethod)
	}
	if authz.Metadata == nil {
		t.Fatal("expected metadata map, got nil")
	}
}
