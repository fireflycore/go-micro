# Registry

`registry` 包定义了服务注册与发现的核心接口与通用模型，供各类注册中心实现复用。

## 核心接口

- **Register**：定义服务注册行为（`Install`、`Uninstall`）。
- **Discovery**：定义服务发现行为（`GetService`、`Watcher`、`Unwatch`、`WatchEvent`），仅网关使用。

## 设计约束

- 业务服务只依赖 `Register`，不依赖 `Discovery`
- `Discovery` 的主职责是维护本地索引，回调订阅属于可选扩展能力
- `registry` 只保留接口与模型，不承载具体适配实现
- etcd、consul、k8s/istio 的实现完全独立维护，互不耦合

## 分层建议

- `go-micro/registry` 保留接口和跨实现共享模型
- `go-etcd/registry`、`go-consul/registry`、`go-k8s/registry` 各自维护实现细节与实现专属模型
- 不把某个注册中心专属字段上提到 `go-micro/registry`
- `ServiceConf`、`GatewayConf` 等实现配置模型下沉到各实现包维护

推荐按下面边界维护：

| 位置 | 放什么 |
|---|---|
| `go-micro/registry` | `Register/Discovery` 接口，`Meta/Network/Kernel/ServiceNode` 等通用模型 |
| `go-etcd/registry` | lease、revision、watch 重连、etcd key 组织等 |
| `go-consul/registry` | check、service meta 编码、blocking query、consul 事件模型等 |
| `go-k8s/registry` | EndpointSlice/Service 查询、K8s 资源监听、Istio 路由映射等 |

不建议把 `go-micro/registry` 完全下沉到实现包，原因如下：

- 会丢失统一契约，业务层会被迫直接依赖某个实现包
- 会出现重复模型，`ServiceNode` 等对象在多个包中无法直接复用
- 会提升迁移成本，从 etcd 切到 consul/k8s 时业务代码改动面会扩大

建议维持“核心契约 + 实现扩展”：

- 核心契约放 `go-micro/registry`
- 实现专属字段放 `go-etcd/go-consul/go-k8s`
- 通过实现包内部转换，避免把实现细节泄漏给业务层

## 实现包
> 注册中心的具体实现位于独立仓库中。

- 基于 etcd v3 的注册与发现实现: `github.com/fireflycore/go-etcd/registry`;
- 基于 Consul 的注册与网关发现实现: `github.com/fireflycore/go-consul/registry`;
- 基于 K8s/Istio 的注册与网关发现实现: `github.com/fireflycore/go-k8s/registry`;

## 迁移设计文档

- Consul 迁移说明：[consul/README.md](file:///Users/lhdht/product/firefly/go-micro/registry/consul/README.md)
- K8s + Istio 迁移说明：[k8s/istio/README.md](file:///Users/lhdht/product/firefly/go-micro/registry/k8s/istio/README.md)
