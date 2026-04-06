package agent

import (
	"context"
	"testing"

	baseregistry "github.com/fireflycore/go-micro/registry"
)

// TestRegistryProviderBuildRegisterRequest 验证 go-micro registry 描述会被正确转换成 register 请求。
func TestRegistryProviderBuildRegisterRequest(t *testing.T) {
	// 创建一个包含 kernel、methods 与 meta 的最小 registry 节点。
	node := &baseregistry.ServiceNode{
		Weight: 80,
		Methods: map[string]bool{
			"/acme.auth.v1.AuthService/Login": true,
		},
		Kernel: &baseregistry.ServiceKernel{
			Language: "go",
			Version:  "go-micro/v1.12.0",
		},
		Meta: &baseregistry.ServiceMeta{
			AppId: "10001",
		},
	}
	// 创建待测 provider。
	provider, err := NewRegistryProvider(RegistryDescriptor{
		ServiceName: "auth",
		Namespace:   "default",
		DNS:         "auth.default.svc.cluster.local",
		Env:         "prod",
		Port:        9090,
		Protocol:    "grpc",
		Version:     "v1.0.0",
		Node:        node,
	})
	if err != nil {
		t.Fatalf("new registry provider failed: %v", err)
	}
	// 构造 register 请求。
	request, err := provider.BuildRegisterRequest(context.Background())
	if err != nil {
		t.Fatalf("build register request failed: %v", err)
	}
	// 核对关键字段是否正确映射。
	if got, want := request.AppID, "10001"; got != want {
		t.Fatalf("unexpected app id: got=%s want=%s", got, want)
	}
	if got, want := request.AppName, "auth"; got != want {
		t.Fatalf("unexpected app name: got=%s want=%s", got, want)
	}
	if got, want := request.Weight, 80; got != want {
		t.Fatalf("unexpected weight: got=%d want=%d", got, want)
	}
	if got, want := len(request.Methods), 1; got != want {
		t.Fatalf("unexpected method count: got=%d want=%d", got, want)
	}
}

// TestNewLocalRuntimeFromRegistry 验证可直接用 go-micro registry 描述组装本地运行时。
func TestNewLocalRuntimeFromRegistry(t *testing.T) {
	// 使用最小 registry 描述直接构造 local runtime。
	runtime, err := NewLocalRuntimeFromRegistry(RegistryDescriptor{
		ServiceName: "payment",
		Namespace:   "default",
		DNS:         "payment.default.svc.cluster.local",
		Env:         "prod",
		Port:        8080,
		Protocol:    "grpc",
		Version:     "v2.0.0",
	}, LocalRuntimeOptions{})
	if err != nil {
		t.Fatalf("new local runtime from registry failed: %v", err)
	}
	// 运行时应成功创建必要组件。
	if runtime.Controller == nil || runtime.Runner == nil {
		t.Fatal("expected local runtime to initialize controller and runner")
	}
}
