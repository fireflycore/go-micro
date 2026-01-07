# Kubernetes Registry

`kubernetes` 子包提供基于 **Kubernetes API** 的服务注册与发现实现，语义与 `etcd` / `consul` 子包保持一致，适合作为在 **K8s + Istio** 场景下的控制面注册中心。

## 设计概览

- 注册侧：
  - 使用一个 ConfigMap 作为存储，名称约定为：`ff-registry-<env>`；
  - ConfigMap 位于 `ServiceConf.Namespace` 对应的 namespace 下；
  - 每个服务实例以一条 `data` 记录存在：
    - key：`<appId>/<leaseId>`；
    - value：序列化后的 `registry.ServiceNode` JSON。
  - `leaseId` 由注册实例本地生成（纳秒时间戳），用于区分不同进程实例。
- 发现侧：
  - 从同一个 ConfigMap 中读取并反序列化所有 `ServiceNode`；
  - 构建两种索引：
    - `service: appId -> []*ServiceNode`；
    - `method: grpcMethod -> appId`（用于 `GetService` 快速定位）。
  - 为模拟 etcd 的 lease 过期语义，会按 `RunDate` + `ServiceConf.TTL` 过滤超时节点，并按 `RunDate` 倒序返回节点（最新心跳优先）。
- 生命周期：
  - `Register.SustainLease` 周期性覆盖写入自身记录，模拟“租约心跳”；
  - `Register.Uninstall` best-effort 删除自己的 key；
  - `Discovery.Watcher` 按 `ServiceConf.TTL/2` 轮询读取 ConfigMap，仅当 `ConfigMap.metadata.resourceVersion` 变化时才重建本地缓存。

## 与 etcd/consul 的一致性

- 接口对齐：
  - 注册实现满足 `registry.Register` 接口；
  - 发现实现满足 `registry.Discovery` 接口。
- 语义对齐：
  - `ServiceNode` 中的 `Meta` / `Network` / `Kernel` 等字段含义与其他后端完全一致；
  - `LeaseId` 作为“实例唯一标识”，在 `kubernetes` 后端同样存在（用于 key 生成和去重）。

只要调用侧只依赖 `registry.Register` / `registry.Discovery` 接口，就可以在 `etcd` / `consul` / `kubernetes` 之间无感切换。

## 基础用法

### In-Cluster 客户端

在 Pod 内访问 Kubernetes APIServer，建议在业务侧自行初始化 client-go clientset，然后注入到本包的 `NewRegister` / `NewDiscover`：

- 使用的环境变量：
  - `KUBERNETES_SERVICE_HOST` / `KUBERNETES_SERVICE_PORT`；
  - 默认 ServiceAccount Token 文件：`/var/run/secrets/kubernetes.io/serviceaccount/token`。

```go
import (
	"github.com/fireflycore/go-micro/registry"
	"github.com/fireflycore/go-micro/registry/kubernetes"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func newK8sClientset() (kubernetes.Interface, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}
```

### 服务注册

```go
func newK8sRegister() (registry.Register, error) {
	client, err := newK8sClientset()
	if err != nil {
		return nil, err
	}

	meta := &registry.Meta{
		Env:     "prod",
		AppId:   "user-service",
		Version: "v1",
	}

	conf := &registry.ServiceConf{
		Namespace: "my-namespace",
		Network: &registry.Network{
			// 建议在 K8s 集群内使用 Service DNS 作为内部地址。
			Internal: "user-service.my-namespace.svc.cluster.local:8080",
		},
		Kernel:   &registry.Kernel{},
		TTL:      10,
		MaxRetry: 3,
	}

	return kubernetes.NewRegister(client, meta, conf)
}
```

上层通常会配合 `registry.NewRegisterService` 使用：

```go
func registerAll(raw []*grpc.ServiceDesc, reg registry.Register) {
	_ = registry.NewRegisterService(raw, reg)
	go reg.SustainLease()
}
```

### 服务发现

```go
func newK8sDiscovery() (registry.Discovery, error) {
	client, err := newK8sClientset()
	if err != nil {
		return nil, err
	}

	meta := &registry.Meta{
		Env: "prod",
	}
	conf := &registry.ServiceConf{
		Namespace: "my-namespace",
	}

	return kubernetes.NewDiscover(client, meta, conf)
}

func exampleDiscover(disc registry.Discovery) {
	go disc.Watcher()

	nodes, err := disc.GetService("/user.UserService/GetProfile")
	_ = nodes
	_ = err
}
```

## Kubernetes + Istio

关于在 Kubernetes + Istio 场景下的使用建议与最终落地方案，见 [K8S_ISTIO.md](file:///d:/project/firefly/go-micro/registry/kubernetes/K8S_ISTIO.md)。

## 与 etcd lease 的差异（重要）

etcd 后端依赖 lease，到期后 key 会被自动删除；而 Kubernetes ConfigMap 不具备 TTL 自动过期能力。因此：

- 若服务进程异常退出未执行 `Uninstall`，ConfigMap 中的旧记录可能残留；
- 本实现通过 `RunDate` 心跳字段模拟“活跃性”，发现侧会按 `RunDate + TTL` 过滤超时节点，避免把流量导向已失活实例；
- 发现侧不会主动清理 ConfigMap 中的陈旧记录（只做读取过滤）。

## 什么时候选择 kubernetes 实现

- 部署环境：
  - 服务已运行在 Kubernetes 集群中；
  - 希望减少额外的外部依赖（如专门部署 etcd / consul 集群）。
- 流量面：
  - 流量全部通过 Kubernetes Service / Istio 管理；
  - 只需要一个轻量级的“服务元信息注册与发现”能力。

如果你已经在使用 etcd/consul 后端，也可以在相同 `registry.Register` / `registry.Discovery` 接口下平滑切换到 `kubernetes`，只需替换初始化部分的构造函数以及服务地址的配置方式即可。
