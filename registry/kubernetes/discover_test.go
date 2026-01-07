package kubernetes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	micro "github.com/fireflycore/go-micro/registry"
)

// fakeRoundTripper 用于拦截 HTTP 请求，返回预设响应，方便在单元测试中模拟 Kubernetes API。
type fakeRoundTripper struct {
	handle func(req *http.Request) (*http.Response, error)
}

func (f *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f.handle(req)
}

// newFakeClient 构造一个基于 httptest.Server 的 Client，便于在测试中覆盖所有请求。
func newFakeClient(t *testing.T, handler http.HandlerFunc, namespace string) *Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	httpClient := &http.Client{
		Transport: srv.Client().Transport,
		Timeout:   5 * time.Second,
	}

	client, err := NewClient(srv.URL, "test-token", namespace, httpClient)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func TestDiscoverRebuildBuildsIndices(t *testing.T) {
	t.Parallel()

	// 构造三类节点：
	// - prod/svcA 与 prod/svcB 应该被缓存；
	// - dev/svcC 应该被过滤。
	nodeA := &micro.ServiceNode{
		LeaseId: 1,
		Meta:    &micro.Meta{Env: "prod", AppId: "svcA"},
		Methods: map[string]bool{"/svcA.Svc/Ping": true},
	}
	nodeB := &micro.ServiceNode{
		LeaseId: 2,
		Meta:    &micro.Meta{Env: "prod", AppId: "svcB"},
		Methods: map[string]bool{"/svcB.Svc/Ping": true},
	}
	nodeOtherEnv := &micro.ServiceNode{
		LeaseId: 3,
		Meta:    &micro.Meta{Env: "dev", AppId: "svcC"},
		Methods: map[string]bool{"/svcC.Svc/Ping": true},
	}

	bA, _ := json.Marshal(nodeA)
	bB, _ := json.Marshal(nodeB)
	bC, _ := json.Marshal(nodeOtherEnv)

	cm := &configMap{
		Metadata: configMapMetadata{
			Name:            configMapName("prod"),
			Namespace:       "test",
			ResourceVersion: "1",
		},
		Data: map[string]string{
			"svcA/1": string(bA),
			"svcB/2": string(bB),
			"svcC/3": string(bC),
			"broken": "not-json",
		},
	}

	ins := &DiscoverInstance{
		meta: &micro.Meta{Env: "prod"},
		conf: &micro.ServiceConf{
			Namespace: "test",
			Network:   &micro.Network{},
			Kernel:    &micro.Kernel{},
			TTL:       10,
			MaxRetry:  3,
		},
		method:  make(micro.ServiceMethod),
		service: make(micro.ServiceDiscover),
	}

	ins.rebuild(cm)

	if len(ins.service) != 2 {
		t.Fatalf("expected 2 services, got %d", len(ins.service))
	}
	if len(ins.method) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(ins.method))
	}

	if _, ok := ins.method["/svcA.Svc/Ping"]; !ok {
		t.Fatalf("method for svcA not found")
	}
	if _, ok := ins.method["/svcB.Svc/Ping"]; !ok {
		t.Fatalf("method for svcB not found")
	}
}

func TestNewDiscoverBootstrapAndWatcher(t *testing.T) {
	t.Parallel()

	// 使用 httptest.Server 模拟 ConfigMap 的两次变化：
	// - 第一次：bootstrap 拉取时返回一个包含单节点的 ConfigMap；
	// - 第二次：Watcher 拉取到包含两个节点的 ConfigMap。
	nodeV1 := &micro.ServiceNode{
		LeaseId: 1,
		Meta:    &micro.Meta{Env: "prod", AppId: "svc"},
		Methods: map[string]bool{"/svc.Svc/Ping": true},
	}
	nodeV2 := &micro.ServiceNode{
		LeaseId: 2,
		Meta:    &micro.Meta{Env: "prod", AppId: "svc"},
		Methods: map[string]bool{"/svc.Svc/Ping": true},
	}

	b1, _ := json.Marshal(nodeV1)
	b2, _ := json.Marshal(nodeV2)

	var call int
	handler := func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/configmaps/"+configMapName("prod")) {
			http.NotFound(w, r)
			return
		}

		call++
		var cm configMap

		switch call {
		case 1:
			// bootstrap 阶段：返回一个节点
			cm = configMap{
				Metadata: configMapMetadata{
					Name:            configMapName("prod"),
					Namespace:       "test",
					ResourceVersion: "1",
				},
				Data: map[string]string{
					"svc/1": string(b1),
				},
			}
		default:
			// Watcher 阶段：返回两个节点（模拟新增实例）
			cm = configMap{
				Metadata: configMapMetadata{
					Name:            configMapName("prod"),
					Namespace:       "test",
					ResourceVersion: "2",
				},
				Data: map[string]string{
					"svc/1": string(b1),
					"svc/2": string(b2),
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&cm)
	}

	client := newFakeClient(t, handler, "test")

	conf := &micro.ServiceConf{
		Namespace: "test",
		Network:   &micro.Network{},
		Kernel:    &micro.Kernel{},
		TTL:       10,
		MaxRetry:  3,
	}

	disc, err := NewDiscover(client, &micro.Meta{Env: "prod"}, conf)
	if err != nil {
		t.Fatalf("new discover: %v", err)
	}

	// bootstrap 之后应该至少有一个节点
	nodes, err := disc.GetService("/svc.Svc/Ping")
	if err != nil {
		t.Fatalf("get service after bootstrap: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node after bootstrap, got %d", len(nodes))
	}

	// 启动 Watcher，等待一次轮询完成。
	// 注意：ServiceConf.Bootstrap 会把 TTL 最小值提升到 10s，因此 Watcher 轮询间隔约为 5s。
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	go disc.Watcher()

	// 轮询等待直到节点数更新为 2 或超时。
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timeout waiting for watcher to update nodes")
		default:
			time.Sleep(100 * time.Millisecond)
			nodes, err := disc.GetService("/svc.Svc/Ping")
			if err != nil {
				continue
			}
			if len(nodes) == 2 {
				disc.Unwatch()
				return
			}
		}
	}
}
