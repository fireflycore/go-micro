package kubernetes

import (
	"context"
	"encoding/json"
	"testing"

	micro "github.com/fireflycore/go-micro/registry"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestRegisterInstallAndUninstall(t *testing.T) {
	t.Parallel()

	client := kubefake.NewSimpleClientset()

	conf := &micro.ServiceConf{
		Namespace: "test",
		Network:   &micro.Network{},
		Kernel:    &micro.Kernel{},
		TTL:       1,
		MaxRetry:  3,
	}

	reg, err := NewRegister(client, &micro.Meta{Env: "prod", AppId: "svc", Version: "v0.0.1"}, conf)
	if err != nil {
		t.Fatalf("new register: %v", err)
	}

	node := &micro.ServiceNode{
		Methods: map[string]bool{"/svc.Svc/Ping": true},
	}

	if err := reg.Install(node); err != nil {
		t.Fatalf("install: %v", err)
	}

	cm, err := client.CoreV1().ConfigMaps("test").Get(context.Background(), configMapName("prod"), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get configmap: %v", err)
	}
	if cm.Data == nil || len(cm.Data) != 1 {
		t.Fatalf("expected 1 entry in configmap data, got %v", cm.Data)
	}

	var raw string
	for _, v := range cm.Data {
		raw = v
	}
	var decoded micro.ServiceNode
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("unmarshal node: %v", err)
	}
	if decoded.Meta == nil || decoded.Meta.AppId != "svc" || decoded.Meta.Env != "prod" {
		t.Fatalf("unexpected decoded meta: %+v", decoded.Meta)
	}
	if decoded.LeaseId == 0 {
		t.Fatalf("expected decoded.LeaseId not zero")
	}

	// Uninstall 应删除该 key（best-effort，但测试期望成功）。
	reg.Uninstall()

	cm, err = client.CoreV1().ConfigMaps("test").Get(context.Background(), configMapName("prod"), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get configmap after uninstall: %v", err)
	}
	if len(cm.Data) != 0 {
		t.Fatalf("expected data to be empty after uninstall, got %d", len(cm.Data))
	}
}
