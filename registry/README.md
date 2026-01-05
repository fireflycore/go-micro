# Registry

`registry` 包定义了服务注册与发现的核心接口，并提供了多种后端实现。

## 核心接口

- **Register**：定义服务注册行为（`Install`、`Uninstall`）。
- **Discovery**：定义服务发现行为（`GetService`、`Watcher`）。

## 支持的后端

本项目支持以下注册中心实现（位于子目录中）：
- **Consul**：基于 HashiCorp Consul
- **Etcd**：基于 Etcd v3
- **Kubernetes**：基于 K8s API

## 使用示例

### 服务注册

```go
import (
	"fmt"

	"github.com/fireflycore/go-micro/registry"
	"github.com/fireflycore/go-micro/registry/consul"
	"github.com/hashicorp/consul/api"
	"google.golang.org/grpc"
)

func ExampleRegister() {
	config := api.DefaultConfig()
	config.Address = "127.0.0.1:8500"
	cli, err := api.NewClient(config)
	if err != nil {
		panic(err)
	}

	reg, err := consul.NewRegister(cli, &registry.Meta{Env: "prod"}, &registry.ServiceConf{})
	if err != nil {
		panic(err)
	}

	var raw []*grpc.ServiceDesc
	errs := registry.NewRegisterService(raw, reg)
	fmt.Println(errs)
}
```

### 服务发现

```go
func ExampleDiscover() {
	cli, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		panic(err)
	}
	disc, err := consul.NewDiscover(cli, &registry.Meta{Env: "prod"}, &registry.ServiceConf{})
	if err != nil {
		panic(err)
	}

	go disc.Watcher()

	nodes, err := disc.GetService("/package.Service/Method")
	fmt.Println(len(nodes), err)
	disc.Unwatch()
}
```
