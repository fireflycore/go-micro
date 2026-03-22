# go-micro/registry Consul 适配约束

> 本文档定义 `go-consul/registry` 在接入 `go-micro/registry` 核心接口时的实现约束。

## 1. 边界

- `go-micro/registry`：接口与模型定义
- `go-consul/registry`：注册与网关发现实现
- 业务服务：仅使用 `Register`
- 网关：使用 `Discovery`

## 2. Register 实现要求

- `Install(*ServiceNode)` 必须补齐并上报：
  - `Meta`（env/app_id/version）
  - `Network`（internal/external/sn）
  - `Kernel`（language/version）
  - `Methods`
- 健康检查必须配置 `DeregisterCriticalServiceAfter`
- `Uninstall()` 必须执行主动注销

## 3. Discovery 实现要求

- `Watcher()` 必须维护两个索引：
  - `Method -> AppId`
  - `AppId -> []*ServiceNode`
- `GetService(method)` 必须仅返回健康实例
- `Unwatch()` 必须可重复调用且幂等
- `WatchEvent(callback)` 作为可选能力，用于向外透传增删改事件

## 4. 数据编码建议

- `methods` 建议 JSON 编码后放入 `Service.Meta`
- 避免将大体积字段放入 `Tags`
- 关键过滤维度（env/version/region）放入 `Meta`

## 5. 官方文档

- Consul Agent Service API  
  <https://developer.hashicorp.com/consul/api-docs/agent/service>
- Consul Health API  
  <https://developer.hashicorp.com/consul/api-docs/health>
