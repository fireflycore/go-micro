# Kubernetes Registry

`kubernetes` 子包提供基于 **Kubernetes API** 的服务注册与发现实现，语义与 `etcd` / `consul` 子包保持一致，适合作为在 **K8s + Istio** 场景下的控制面注册中心。

## 设计概览

- 注册侧：
  - 使用一个 ConfigMap 作为存储，名称约定为：`ff-registry-<env>`；
  - ConfigMap 位于 `ServiceConf.Namespace` 对应的 namespace 下；
  - 每个服务实例以一条 `data` 记录存在：
    - key：`<appId>/<leaseId>`；
    - value：序列化后的 `registry.ServiceNode` JSON。
- 发现侧：
  - 从同一个 ConfigMap 中读取并反序列化所有 `ServiceNode`；
  - 构建两种索引：
    - `service: appId -> []*ServiceNode`；
    - `method: grpcMethod -> appId`（用于 `GetService` 快速定位）。
- 生命周期：
  - `Register.SustainLease` 周期性覆盖写入自身记录，模拟“租约心跳”；
  - `Register.Uninstall` 删除自己的 key；
  - `Discovery.Watcher` 以 `ConfigMap.metadata.resourceVersion` 为游标轮询更新本地缓存。

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
			// 建议在 K8s 集群内使用 Service DNS 作为内部地址（见下一节 Istio 建议）。
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

## 在 Kubernetes + Istio 场景下的使用建议

本实现默认只负责“服务元数据存储与查询”，并不直接操控 Istio CRD。要在企业级 **K8s + Istio** 部署中使用，建议遵循以下约定，使两者协同工作而不是互相干扰。

### 1. 地址：使用 Service DNS，而不是 Pod IP

- `ServiceConf.Network.Internal`：
  - 同命名空间：`<svc-name>:<port>`，例如 `user-service:8080`；
  - 跨命名空间：`<svc>.<ns>.svc.cluster.local:<port>`。
- `ServiceConf.Network.External`（可选）：
  - 如果需要给集群外调用方使用，可以配置为 IngressGateway 或外部负载均衡地址。

这样，调用方拿到 `ServiceNode` 后：

- 使用 `Network.Internal` 建立 gRPC 连接；
- 连接目标其实是 Kubernetes Service，流量会按常规路径进入 sidecar，由 Istio 按 VirtualService / DestinationRule 做路由和负载均衡；
- registry 不绕过 mesh，而是“告诉你应该连哪一个 Service”。

### 2. 元信息：与 Pod Label / DestinationRule 对齐

统一以下约定可以让 registry 的元信息与 Istio 配置自然对齐：

- `Meta.AppId`：
  - 建议与 Pod / Deployment 的 `app` label 一致；
  - 例如 `app=user-service`。
- `Meta.Env`：
  - 建议反映环境信息，可与命名空间或 label `env=prod` 对应；
  - 同一 env 的服务会共享同一个 ConfigMap。
- `Meta.Version`：
  - 建议与 Pod label `version` 或你在 DestinationRule 中使用的 `subset` 语义保持一致；
  - 例如 `version=v1` / `v2`，对应 `subset: v1` / `subset: v2`。

在此基础上：

- registry 提供的 `ServiceNode` 中已经包含 `Meta.Version` 等信息；
- 上层可以根据版本/区域等信息决定要打到哪个 subset，并通过 HTTP Header/gRPC metadata 与 Istio 的路由规则结合，实现灰度发布、AB 测试等。

### 3. 控制面访问与 mTLS

client-go clientset 访问的是 Kubernetes APIServer：

- 使用 in-cluster 配置和 ServiceAccount Token；
- 在大多数 Istio 部署里，访问 APIServer 通常在 mesh 排除列表中；
- 因此：
  - 注册/发现侧访问 ConfigMap 的 HTTP 调用不会被 sidecar 拦截；
  - 不会受到 mesh mTLS 及流量规则影响。

这意味着本模块可以安全地作为“控制面元信息存储”，而数据面流量由 Istio 全权接管。

## 什么时候选择 kubernetes 实现

- 部署环境：
  - 服务已运行在 Kubernetes 集群中；
  - 希望减少额外的外部依赖（如专门部署 etcd / consul 集群）。
- 流量面：
  - 流量全部通过 Kubernetes Service / Istio 管理；
  - 只需要一个轻量级的“服务元信息注册与发现”能力。

如果你已经在使用 etcd/consul 后端，也可以在相同 `registry.Register` / `registry.Discovery` 接口下平滑切换到 `kubernetes`，只需替换初始化部分的构造函数以及服务地址的配置方式即可。
