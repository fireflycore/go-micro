package kubernetes

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	micro "github.com/fireflycore/go-micro/registry"
)

type configMapStore struct {
	mu              sync.Mutex
	resourceVersion int
	cm              *configMap
}

func (s *configMapStore) bumpRV() string {
	s.resourceVersion++
	return intToString(s.resourceVersion)
}

func intToString(v int) string {
	return strconv.Itoa(v)
}

func TestRegisterInstallAndUninstall(t *testing.T) {
	t.Parallel()

	// 用内存 store 模拟 Kubernetes API 上的 ConfigMap 存储。
	store := &configMapStore{
		resourceVersion: 0,
		cm:              nil,
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		// 仅处理本测试需要的三个 API：
		// - GET  configmaps/{name}
		// - POST configmaps
		// - PATCH configmaps/{name}
		if !strings.Contains(r.URL.Path, "/api/v1/namespaces/test/configmaps") {
			http.NotFound(w, r)
			return
		}

		store.mu.Lock()
		defer store.mu.Unlock()

		switch r.Method {
		case http.MethodGet:
			if store.cm == nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(store.cm)
		case http.MethodPost:
			b, _ := io.ReadAll(r.Body)
			_ = r.Body.Close()

			var cm configMap
			if err := json.Unmarshal(b, &cm); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			cm.Metadata.Namespace = "test"
			cm.Metadata.ResourceVersion = store.bumpRV()
			if cm.Data == nil {
				cm.Data = map[string]string{}
			}

			store.cm = &cm
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(&cm)
		case http.MethodPatch:
			if store.cm == nil {
				http.NotFound(w, r)
				return
			}

			b, _ := io.ReadAll(r.Body)
			_ = r.Body.Close()

			var patch map[string]any
			if err := json.Unmarshal(b, &patch); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			dataAny, ok := patch["data"].(map[string]any)
			if !ok {
				http.Error(w, "patch missing data", http.StatusBadRequest)
				return
			}

			if store.cm.Data == nil {
				store.cm.Data = map[string]string{}
			}

			for k, v := range dataAny {
				if v == nil {
					delete(store.cm.Data, k)
					continue
				}
				store.cm.Data[k] = v.(string)
			}

			store.cm.Metadata.ResourceVersion = store.bumpRV()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(store.cm)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL, "test-token", "test", srv.Client())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

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

	// 校验 ConfigMap.data 已写入一个 key（svc/<leaseId>）。
	store.mu.Lock()
	if store.cm == nil || len(store.cm.Data) != 1 {
		store.mu.Unlock()
		t.Fatalf("expected 1 entry in configmap data, got %v", store.cm)
	}
	store.mu.Unlock()

	// Uninstall 应删除该 key（best-effort，但测试期望成功）。
	reg.Uninstall()

	// 等待 PATCH 执行完成（Uninstall 内部有超时与网络调用）。
	time.Sleep(50 * time.Millisecond)

	store.mu.Lock()
	defer store.mu.Unlock()

	if store.cm == nil {
		t.Fatalf("expected configmap to exist")
	}
	if len(store.cm.Data) != 0 {
		t.Fatalf("expected data to be empty after uninstall, got %d", len(store.cm.Data))
	}
}
