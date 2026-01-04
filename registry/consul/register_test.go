package consul

import (
	"os"
	"testing"
	"time"

	micro "github.com/fireflycore/go-micro/registry"
	"github.com/hashicorp/consul/api"
)

func TestRegister(t *testing.T) {
	address := os.Getenv("CONSUL_ADDRESS")
	if address == "" {
		t.Skip("CONSUL_ADDRESS is empty")
	}

	config := api.DefaultConfig()
	config.Address = address
	cli, err := api.NewClient(config)
	if err != nil {
		t.Fatal(err)
	}

	serviceConf := &micro.ServiceConf{
		Network: &micro.Network{
			SN:       "test",
			Internal: "127.0.0.1",
			External: "127.0.0.1",
		},
		Kernel:    &micro.Kernel{},
		Namespace: "test-namespace",
		TTL:       10,
		MaxRetry:  0,
	}

	meta := &micro.Meta{
		AppId:   "test-service",
		Env:     "prod",
		Version: "v0.0.1",
	}

	reg, err := NewRegister(cli, meta, serviceConf)
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Uninstall()

	service := &micro.ServiceNode{
		Methods: map[string]bool{
			"/test.Service/Ping": true,
		},
	}

	if err := reg.Install(service); err != nil {
		t.Fatal(err)
	}

	go reg.SustainLease()
	time.Sleep(100 * time.Millisecond)
}
