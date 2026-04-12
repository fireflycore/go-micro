# Baremetal Registry Index

这个目录现在只作为 **裸机 sidecar-agent 主路径的文档索引与迁移入口**。

当前 Firefly 裸机运行时已经收敛为：

```text
业务服务
  → go-consul/agent
  → 本机 sidecar-agent
  → sidecar-agent 对接 consul / envoy
```

## 当前约定

- 裸机业务服务统一接入 [agent](file:///Users/lhdht/product/firefly/go-consul/agent/README.md)
- `go-micro/invocation` 仍是调用主路径
- K8s 不复用这套裸机注册/摘流/注销语义
- 根目录不再承载旧的 `Register / Discovery / ServiceNode` 抽象代码

## 目录说明

| 位置 | 说明 |
|---|---|
| `go-micro/registry` | 文档索引与迁移入口 |
| `go-consul/agent` | 裸机 sidecar-agent 正式接入库 |

## 后续迁移

- 面向业务服务的新接入，一律走 `go-consul/agent`
- 根目录历史 API 已删除后，不再作为 Go 包导出使用
- 若需要批量改仓，可参考 [MIGRATION.md](file:///Users/lhdht/product/firefly/go-micro/registry/MIGRATION.md)
