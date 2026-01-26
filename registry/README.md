# Registry

`registry` 包定义了服务注册与发现的核心接口与通用模型，供各类注册中心实现复用。

## 核心接口

- **Register**：定义服务注册行为（`Install`、`Uninstall`）。
- **Discovery**：定义服务发现行为（`GetService`、`Watcher`）。

## 实现包
> 注册中心的具体实现位于独立仓库中。

- 基于 etcd v3 的注册与发现实现: `github.com/fireflycore/go-etcd/registry`;
