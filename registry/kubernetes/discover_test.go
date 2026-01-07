package kubernetes

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	micro "github.com/fireflycore/go-micro/registry"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestDiscoverRebuildBuildsIndices(t *testing.T) {
	t.Parallel()

	// 构造三类节点：
	// - prod/svcA 与 prod/svcB 应该被缓存；
	// - dev/svcC 应该被过滤。
	nodeA := &micro.ServiceNode{
		LeaseId: 1,
		Meta:    &micro.Meta{Env: "prod", AppId: "svcA"},
		RunDate: time.Now().Format(time.DateTime),
		Methods: map[string]bool{"/svcA.Svc/Ping": true},
	}
	nodeB := &micro.ServiceNode{
		LeaseId: 2,
		Meta:    &micro.Meta{Env: "prod", AppId: "svcB"},
		RunDate: time.Now().Format(time.DateTime),
		Methods: map[string]bool{"/svcB.Svc/Ping": true},
	}
	nodeOtherEnv := &micro.ServiceNode{
		LeaseId: 3,
		Meta:    &micro.Meta{Env: "dev", AppId: "svcC"},
		RunDate: time.Now().Format(time.DateTime),
		Methods: map[string]bool{"/svcC.Svc/Ping": true},
	}

	bA, _ := json.Marshal(nodeA)
	bB, _ := json.Marshal(nodeB)
	bC, _ := json.Marshal(nodeOtherEnv)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
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

	ins.rebuild(cm, cm.ResourceVersion)

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

func TestDiscoverRebuildFiltersStaleNodesAndSorts(t *testing.T) {
	t.Parallel()

	now := time.Now()

	nodeStale := &micro.ServiceNode{
		LeaseId: 1,
		Meta:    &micro.Meta{Env: "prod", AppId: "svc"},
		RunDate: now.Add(-20 * time.Second).Format(time.DateTime),
		Methods: map[string]bool{"/svc.Svc/Ping": true},
	}
	nodeOlder := &micro.ServiceNode{
		LeaseId: 2,
		Meta:    &micro.Meta{Env: "prod", AppId: "svc"},
		RunDate: now.Add(-2 * time.Second).Format(time.DateTime),
		Methods: map[string]bool{"/svc.Svc/Ping": true},
	}
	nodeNewer := &micro.ServiceNode{
		LeaseId: 3,
		Meta:    &micro.Meta{Env: "prod", AppId: "svc"},
		RunDate: now.Add(-1 * time.Second).Format(time.DateTime),
		Methods: map[string]bool{"/svc.Svc/Ping": true},
	}

	b1, _ := json.Marshal(nodeStale)
	b2, _ := json.Marshal(nodeOlder)
	b3, _ := json.Marshal(nodeNewer)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            configMapName("prod"),
			Namespace:       "test",
			ResourceVersion: "1",
		},
		Data: map[string]string{
			"svc/1": string(b1),
			"svc/2": string(b2),
			"svc/3": string(b3),
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

	ins.rebuild(cm, cm.ResourceVersion)

	nodes := ins.service["svc"]
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes after filtering, got %d", len(nodes))
	}
	if nodes[0].LeaseId != 3 {
		t.Fatalf("expected newest node first, got leaseId=%d", nodes[0].LeaseId)
	}
	if nodes[1].LeaseId != 2 {
		t.Fatalf("expected older node second, got leaseId=%d", nodes[1].LeaseId)
	}
	if _, ok := ins.method["/svc.Svc/Ping"]; !ok {
		t.Fatalf("expected method index to exist")
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
		RunDate: time.Now().Format(time.DateTime),
		Methods: map[string]bool{"/svc.Svc/Ping": true},
	}
	nodeV2 := &micro.ServiceNode{
		LeaseId: 2,
		Meta:    &micro.Meta{Env: "prod", AppId: "svc"},
		RunDate: time.Now().Format(time.DateTime),
		Methods: map[string]bool{"/svc.Svc/Ping": true},
	}

	b1, _ := json.Marshal(nodeV1)
	b2, _ := json.Marshal(nodeV2)

	client := kubefake.NewSimpleClientset()
	ns := "test"
	name := configMapName("prod")

	_, err := client.CoreV1().ConfigMaps(ns).Create(context.Background(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Data: map[string]string{
			"svc/1": string(b1),
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create configmap: %v", err)
	}

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

	go func() {
		time.Sleep(200 * time.Millisecond)

		cm, err := client.CoreV1().ConfigMaps(ns).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return
		}
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		cm.Data["svc/2"] = string(b2)
		cm.ResourceVersion = "2"
		_, _ = client.CoreV1().ConfigMaps(ns).Update(context.Background(), cm, metav1.UpdateOptions{})
	}()

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
