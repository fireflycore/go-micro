# Kubernetes / Istio Registry

在云原生（Istio + Kubernetes）架构下，服务注册与发现模式发生了根本性转变：

1.  **去中心化注册**：应用不再需要主动调用 API 写入注册中心（如 ETCD/Consul）。应用只需通过 Kubernetes Probe 暴露健康状态，Endpoint Controller 会自动维护可用 Pod 列表。
2.  **DNS/Sidecar 发现**：客户端不再需要 Watch 注册中心。客户端只需 Dial 目标服务的 K8S Service Name（DNS），流量由 K8S Service iptables 或 Istio Envoy Sidecar 拦截并路由。

因此，本模块实现了“极简”的注册与发现逻辑。

## 核心概念：注册与发现的分离

**不是每个服务都需要同时使用 `NewRegister` 和 `NewDiscovery`。**

*   **服务端（Server）角色**：只需要 `NewRegister`。
    *   作用：暴露健康检查（Health Check），告诉 K8S “我准备好接客了”。
*   **客户端（Client）角色**：只需要 `NewDiscovery`。
    *   作用：生成目标地址（DNS），告诉 gRPC “我要去访问谁”。

---

## 最佳实践：端口策略

在 Kubernetes 环境中，由于容器拥有独立的网络栈，不同服务使用相同的容器端口（Container Port）**不会冲突**。

**我们强烈建议采用“统一端口”策略：**
*   所有微服务的 gRPC 容器端口统一（例如 `9090`）。
*   所有 K8S Service 的暴露端口统一（例如 `9090`）。

**优势：**
*   **运维简单**：Pod 模板、Service YAML、防火墙规则可以标准化。
*   **开发简单**：客户端发现器只需配置一次默认端口，即可访问所有服务。

更多大型项目实战案例（如多命名空间、异构端口管理），请参考 [K8S_ISTIO.md](../../../../ops/K8S_ISTIO.md)。

---

## 进阶：如何处理异构端口？

如果你的项目中确实存在不同端口的服务（例如：User用9090，Storage用8080，Redis用6379），`Discovery` 提供了 **PortMap** 机制来优雅管理。

你不需要为每个服务创建 `Discovery`，只需在初始化时配置映射表：

```go
func Init() {
    // 全局创建一个发现器
    // 1. 设置默认端口为 9090 (绝大多数服务使用)
    // 2. 为特殊服务配置端口映射
    discovery = kubernetes.NewDiscovery("default", 9090, kubernetes.WithPortMap(map[string]int{
        "storage": 8080,
        "redis":   6379,
        "mysql":   3306,
    }))
}

func Call() {
    // 1. 调用 User (命中默认端口 9090)
    // -> "dns:///user.default:9090"
    t1 := discovery.GetTarget("user")
    
    // 2. 调用 Storage (命中映射表 8080)
    // -> "dns:///storage.default:8080"
    t2 := discovery.GetTarget("storage")
    
    // 3. 临时覆盖 (显式指定优先)
    // -> "dns:///storage.default:8081"
    t3 := discovery.GetTarget("storage:8081")
}
```

这样，你依然只需要维护**一个全局 Discovery 实例**，即可优雅地处理所有服务的调用。

---

## Register (服务端)

`NewRegister` 仅负责启动标准 gRPC Health Check 服务与 Reflection 反射服务。

```go
import (
    "google.golang.org/grpc"
    "github.com/fireflycore/go-micro/registry/kubernetes"
)

func main() {
    s := grpc.NewServer()
    
    // 1. 创建注册器（自动挂载 Health Server & Reflection）
    // 支持通过环境变量 K8S_SHUTDOWN_WAIT 控制优雅停机等待时间
    reg := kubernetes.NewRegister(s)
    
    // 2. 注册资源清理钩子（可选）
    reg.OnShutdown(func() {
        // 关闭 DB/Redis 等
    })
    
    // 3. 注册业务服务
    pb.RegisterGreeterServer(s, &server{})
    
    // 4. 启动（标记 Health 为 SERVING）
    reg.Start()
    
    // 5. 阻塞运行，托管信号与优雅停机流程
    go func() {
        s.Serve(lis)
    }()
    reg.RunBlock()
}
```

在 K8S Deployment YAML 中配置 Probe：

```yaml
readinessProbe:
  grpc:
    port: 9090
  initialDelaySeconds: 5
```

## Discovery (客户端)

`Discovery` 支持智能解析，既支持统一端口，也兼容特殊端口。

```go
import (
    "github.com/fireflycore/go-micro/registry/kubernetes"
)

func main() {
    // 全局创建一个发现器（配置团队约定的默认 Namespace 和 端口）
    disc := kubernetes.NewDiscovery("default", 9090)
    
    // 场景 1：标准服务（使用默认端口 9090）
    // 生成: "dns:///user-service.default:9090"
    target1 := disc.GetTarget("user-service")
    
    // 场景 2：特殊服务（覆盖端口）
    // 生成: "dns:///payment-service.default:8080"
    target2 := disc.GetTarget("payment-service:8080")
    
    // 场景 3：跨命名空间（覆盖 Namespace）
    // 生成: "dns:///redis.system:6379"
    target3 := disc.GetTarget("redis.system:6379")
    
    // 建立连接 (建议开启 RoundRobin)
    conn, _ := grpc.Dial(target1, 
        grpc.WithInsecure(),
        grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
    )
}
```
