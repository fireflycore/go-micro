# go-micro/registry K8s + Istio 适配约束

> 本文档定义 `go-k8s/registry` 在最终态中的接口实现约束。

## 1. 目标

- 服务注册与健康探测完全下沉到 K8s
- 流量治理、灰度、故障转移交给 Istio
- `go-micro/registry` 仅保持调用面稳定

## 2. Register 实现要求

- `Install(*ServiceNode)` 在 K8s 环境默认 No-op
- `Uninstall()` 在 K8s 环境默认 No-op
- 不允许在业务服务进程内做额外注册中心依赖初始化

## 3. Discovery 实现要求（仅网关）

- 支持两种返回策略：
  - 返回 Service FQDN，网关直连 Service
  - 返回 EndpointSlice 实例列表，网关做节点级策略
- `Watch(ctx)` 监听 K8s 资源变化时必须支持断线重连
- `GetService(method)` 需要结合路由配置完成 method 到 service 的映射

## 4. 路由建议

- method 到 service 的映射建议放入可热更新配置
- Istio 侧通过 VirtualService/DestinationRule 管理版本流量
- 网关只做鉴权和入口策略，不重复实现 mesh 能力

## 5. 官方文档

- Kubernetes Service  
  <https://kubernetes.io/docs/concepts/services-networking/service/>
- Kubernetes DNS  
  <https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/>
- Kubernetes Probes  
  <https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/>
- Istio Traffic Management  
  <https://istio.io/latest/docs/concepts/traffic-management/>
- Istio VirtualService  
  <https://istio.io/latest/docs/reference/config/networking/virtual-service/>
