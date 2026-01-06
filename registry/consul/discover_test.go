package consul

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	micro "github.com/fireflycore/go-micro/registry"
	"github.com/hashicorp/consul/api"
)

func TestDiscoverRebuildBuildsIndices(t *testing.T) {
	t.Parallel()

	ins := &DiscoverInstance{
		meta:    &micro.Meta{Env: "prod"},
		conf:    &micro.ServiceConf{Namespace: "test"},
		method:  make(micro.ServiceMethod),
		service: make(micro.ServiceDiscover),
	}

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

	ins.rebuild([]*api.ServiceEntry{
		{Service: &api.AgentService{Meta: map[string]string{consulMetaKeyNode: string(bA)}}},
		{Service: &api.AgentService{Meta: map[string]string{consulMetaKeyNode: string(bB)}}},
		{Service: &api.AgentService{Meta: map[string]string{consulMetaKeyNode: string(bC)}}},
		{Service: &api.AgentService{Meta: map[string]string{consulMetaKeyNode: "not-json"}}},
		{Service: &api.AgentService{Meta: map[string]string{}}},
		nil,
	})

	nodes, err := ins.GetService("/svcA.Svc/Ping")
	if err != nil {
		t.Fatalf("get service: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Meta == nil || nodes[0].Meta.AppId != "svcA" {
		t.Fatalf("unexpected node meta: %+v", nodes[0].Meta)
	}

	if _, err := ins.GetService("/svcC.Svc/Ping"); err == nil {
		t.Fatalf("expected env-mismatch method not found")
	}
}

func TestNewDiscoverBootstrapUsesHealthService(t *testing.T) {
	t.Parallel()

	rawNode, err := json.Marshal(&micro.ServiceNode{
		LeaseId: 1,
		Meta:    &micro.Meta{Env: "prod", AppId: "svc"},
		Methods: map[string]bool{"/svc.Svc/Ping": true},
	})
	if err != nil {
		t.Fatalf("marshal node: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/v1/health/service/") {
			http.NotFound(w, r)
			return
		}
		if strings.TrimPrefix(r.URL.Path, "/v1/health/service/") != "test-prod" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Consul-Index", "7")
		_, _ = w.Write([]byte(`[{"Service":{"Meta":{"` + consulMetaKeyNode + `":` + string(jsonString(string(rawNode))) + `}}}]`))
	}))
	defer srv.Close()

	parsed, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	cfg := api.DefaultConfig()
	cfg.Scheme = parsed.Scheme
	cfg.Address = parsed.Host
	cfg.HttpClient = srv.Client()
	cli, err := api.NewClient(cfg)
	if err != nil {
		t.Fatalf("new consul client: %v", err)
	}

	disc, err := NewDiscover(cli, &micro.Meta{Env: "prod"}, &micro.ServiceConf{Namespace: "test"})
	if err != nil {
		t.Fatalf("new discover: %v", err)
	}

	nodes, err := disc.GetService("/svc.Svc/Ping")
	if err != nil {
		t.Fatalf("get service: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
}

func jsonString(s string) []byte {
	b, _ := json.Marshal(s)
	return b
}

