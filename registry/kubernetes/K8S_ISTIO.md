# Kubernetes + Istio 方案

本文档描述 `registry/kubernetes` 在 Kubernetes + Istio 场景下的推荐用法与最终落地方案。该方案的核心是职责拆分：数据面由 Istio 负责，registry 只承担控制面元信息存储与方法索引能力。

## 目标与职责边界

- 数据面（流量治理）：
  - 连接目标使用 Kubernetes Service（建议通过 Service DNS）；
  - 负载均衡、灰度发布、熔断限流等由 Istio（VirtualService / DestinationRule）管理；
  - registry 不绕过 Service 与 sidecar，不做实例级负载均衡决策。
- 控制面（元信息与路由索引）：
  - 使用 ConfigMap 存储 `ServiceNode(JSON)`，并在发现侧构建 `method -> appId -> nodes` 的本地索引；
  - 用于网关/客户端做方法级定位、快速校验（例如方法是否存在、归属哪个服务）。

## 使用建议

### 1. 地址：使用 Service DNS，而不是 Pod IP

- `ServiceConf.Network.Internal`：
  - 同命名空间：`<svc-name>:<port>`，例如 `user-service:8080`；
  - 跨命名空间：`<svc>.<ns>.svc.cluster.local:<port>`。
- `ServiceConf.Network.External`（可选）：
  - 如需提供集群外访问，可配置为 IngressGateway 或外部负载均衡地址。

调用方从 registry 拿到 `ServiceNode` 后：

- 使用 `Network.Internal` 建立 gRPC 连接；
- 实际连接目标是 Kubernetes Service，流量进入 sidecar，再由 Istio 路由与负载均衡；
- registry 的作用是“告诉你应该连哪个 Service”，而不是“告诉你连哪个 Pod”。

### 2. 元信息：与 Pod Label / DestinationRule 对齐

建议统一以下约定，使 registry 元信息与 Istio 配置自然对齐：

- `Meta.AppId`：
  - 建议与 Kubernetes Service 名称、Deployment/Pod 的 `app` label 一致；
  - 例如 `app=user-service`。
- `Meta.Env`：
  - 用于环境隔离，与命名空间或 label `env=prod` 对应；
  - 同一 env 的服务共享同一个 ConfigMap（`ff-registry-<env>`）。
- `Meta.Version`：
  - 建议与 Pod `version` label 或 DestinationRule 的 `subsets[].name` 一致；
  - 例如 `v1` / `v2`，对应 Istio `subset: v1` / `subset: v2`。

在此基础上，上层可以根据版本/区域等信息决定要打到哪个 subset，并通过 HTTP Header / gRPC metadata 与 Istio 路由规则结合，实现灰度发布、AB 测试等。

### 3. 控制面访问与 mTLS

注册/发现侧使用 client-go 访问 Kubernetes APIServer：

- 使用 in-cluster 配置和 ServiceAccount Token；
- 在多数 Istio 部署中，访问 APIServer 通常不受数据面规则影响；
- 因此注册/发现对 ConfigMap 的访问不会被 sidecar 以数据面策略拦截。

## 最终落地方案（建议）

以 Kubernetes + Istio 为最终方案时，推荐：

- 服务实例启动：
  - `Install` 写入节点信息；
  - `SustainLease` 周期性更新 `RunDate`（心跳），用于发现侧过滤失活节点。
- 网关/调用方启动：
  - `NewDiscover` 先拉取快照；
  - `Watcher` 按轮询刷新本地索引；
  - `GetService` 返回节点列表时使用最新心跳优先策略。

## 现实约束与后续演进方向

- ConfigMap 存储容量与写入冲突：实例/方法数量上升会带来对象膨胀与 Update 冲突重试，需关注 APIServer 压力与数据规模。
- “实例存活”语义：Kubernetes 没有 etcd lease 的自动删除能力，异常退出可能残留旧记录；发现侧通过 `RunDate + TTL` 过滤失活节点规避误路由（但不会主动清理 ConfigMap）。
- 可选的进一步收敛：如果未来希望把实例级负载均衡完全交给 Istio，可将 registry 的输出收敛为 `method -> appId/ServiceName`，减少节点级数据与写入频率。
