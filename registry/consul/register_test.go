package consul

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	micro "github.com/fireflycore/go-micro/registry"
	"github.com/hashicorp/consul/api"
)

func TestRegisterInstallAndUninstall(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var gotRegister *api.AgentServiceRegistration
	var gotUpdateCheckIDs []string
	var gotDeregisterIDs []string
	var gotRequests []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimRight(r.URL.Path, "/")
		mu.Lock()
		gotRequests = append(gotRequests, r.Method+" "+path)
		mu.Unlock()

		switch {
		case path == "/v1/agent/service/register":
			var reg api.AgentServiceRegistration
			if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
				t.Fatalf("decode register body: %v", err)
			}
			mu.Lock()
			gotRegister = &reg
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return

		case strings.HasPrefix(path, "/v1/agent/check/update/"):
			checkID := strings.TrimPrefix(path, "/v1/agent/check/update/")
			mu.Lock()
			gotUpdateCheckIDs = append(gotUpdateCheckIDs, checkID)
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return

		case strings.HasPrefix(path, "/v1/agent/service/deregister/"):
			serviceID := strings.TrimPrefix(path, "/v1/agent/service/deregister/")
			mu.Lock()
			gotDeregisterIDs = append(gotDeregisterIDs, serviceID)
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}

		http.NotFound(w, r)
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

	conf := &micro.ServiceConf{
		Namespace: "test",
		Network: &micro.Network{
			SN:       "sn",
			Internal: "127.0.0.1",
			External: "127.0.0.1",
		},
		Kernel: &micro.Kernel{},
		TTL:    10,
	}

	reg, err := NewRegister(cli, &micro.Meta{Env: "prod", AppId: "svc", Version: "v0.0.1"}, conf)
	if err != nil {
		t.Fatalf("new register: %v", err)
	}

	node := &micro.ServiceNode{
		Methods: map[string]bool{"/svc.Svc/Ping": true},
	}

	if err := reg.Install(node); err != nil {
		mu.Lock()
		reqs := append([]string(nil), gotRequests...)
		mu.Unlock()
		t.Fatalf("install: %v, requests=%v", err, reqs)
	}

	mu.Lock()
	registered := gotRegister
	updateCheckIDs := append([]string(nil), gotUpdateCheckIDs...)
	mu.Unlock()

	if registered == nil {
		t.Fatalf("expected service register called")
	}
	if registered.ID == "" {
		t.Fatalf("expected registered.ID not empty")
	}
	if registered.Name != "test-prod" {
		t.Fatalf("expected registered.Name=test-prod, got %q", registered.Name)
	}
	if registered.Address != "127.0.0.1" {
		t.Fatalf("expected registered.Address=127.0.0.1, got %q", registered.Address)
	}
	if registered.Check == nil {
		t.Fatalf("expected registered.Check not nil")
	}
	if registered.Check.TTL != "10s" {
		t.Fatalf("expected check.TTL=10s, got %q", registered.Check.TTL)
	}
	if registered.Check.DeregisterCriticalServiceAfter != "30s" {
		t.Fatalf("expected check.DeregisterCriticalServiceAfter=30s, got %q", registered.Check.DeregisterCriticalServiceAfter)
	}
	if registered.Meta == nil {
		t.Fatalf("expected registered.Meta not nil")
	}
	if registered.Meta[consulMetaKeyAppID] != "svc" {
		t.Fatalf("expected meta appId=svc, got %q", registered.Meta[consulMetaKeyAppID])
	}
	if registered.Meta[consulMetaKeyEnv] != "prod" {
		t.Fatalf("expected meta env=prod, got %q", registered.Meta[consulMetaKeyEnv])
	}
	if registered.Meta[consulMetaKeyVersion] != "v0.0.1" {
		t.Fatalf("expected meta version=v0.0.1, got %q", registered.Meta[consulMetaKeyVersion])
	}
	rawNode := registered.Meta[consulMetaKeyNode]
	if rawNode == "" {
		t.Fatalf("expected meta node not empty")
	}
	var decoded micro.ServiceNode
	if err := json.Unmarshal([]byte(rawNode), &decoded); err != nil {
		t.Fatalf("unmarshal meta node: %v", err)
	}
	if decoded.Meta == nil || decoded.Meta.AppId != "svc" || decoded.Meta.Env != "prod" {
		t.Fatalf("unexpected decoded meta: %+v", decoded.Meta)
	}
	if decoded.LeaseId == 0 {
		t.Fatalf("expected decoded.LeaseId not zero")
	}
	if decoded.Network == nil || decoded.Network.Internal != "127.0.0.1" {
		t.Fatalf("unexpected decoded network: %+v", decoded.Network)
	}

	if node.LeaseId == 0 {
		t.Fatalf("expected node.LeaseId not zero")
	}
	if node.Meta == nil || node.Meta.AppId != "svc" || node.Meta.Env != "prod" {
		t.Fatalf("unexpected node meta: %+v", node.Meta)
	}

	if len(updateCheckIDs) == 0 {
		t.Fatalf("expected UpdateTTL called at least once")
	}
	if updateCheckIDs[0] != "service:"+registered.ID {
		t.Fatalf("expected checkID=%q, got %q", "service:"+registered.ID, updateCheckIDs[0])
	}

	reg.Uninstall()

	mu.Lock()
	deregisterIDs := append([]string(nil), gotDeregisterIDs...)
	mu.Unlock()

	if len(deregisterIDs) != 1 {
		t.Fatalf("expected 1 deregister call, got %d", len(deregisterIDs))
	}
	if deregisterIDs[0] != registered.ID {
		t.Fatalf("expected deregister id=%q, got %q", registered.ID, deregisterIDs[0])
	}
}
