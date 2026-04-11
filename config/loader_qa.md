# Loader QA

## 为什么 `LoadConfig` / `LoadStoreConfig` 的传参没有继续收敛为一个统一接口？

### 问题

`LoadConfig` 当前签名同时接收：

- 参数对象：`LoaderParams`
- 行为依赖：`LocalLoaderFunc` / `RemoteLoaderFunc` / `PayloadDecodeFunc`

看起来似乎可以进一步收敛成一个接口。

### 结论

当前阶段不建议把这些入参进一步收敛为单一接口。

### 原因

1. `params` 和 `loader/decode` 的职责不同  
   `LoaderParams` 描述的是“这次要加载什么配置”；`LocalLoaderFunc`、`RemoteLoaderFunc`、`PayloadDecodeFunc` 描述的是“这次如何加载”。两者是数据与策略的关系，直接混在一个对象里会弱化边界。

2. 这些依赖本身很轻  
   `localLoad`、`remoteLoad`、`payloadDecode` 都是一次性注入的轻量函数，没有复杂状态，也没有需要长期持有的生命周期。为了它们单独抽接口，收益较小。

3. `Store` 已经是稳定抽象  
   `LoadStoreConfig` 依赖的 `store` 已经通过 `Store` 接口抽象完成。真正适合做接口的是这种“后端实现可替换”的能力，而不是几个简单函数签名。

4. 保持顶层泛型函数更自然  
   当前使用的是泛型顶层函数 `LoadConfig[T any]`、`LoadStoreConfig[T any]`。在这里继续引入一层统一接口，不会明显提升可读性，反而容易让“请求参数”和“加载依赖”的职责混在一起。

5. 测试成本更低  
   现在的调用方式可以直接传匿名函数或轻量闭包，测试时非常直接，不需要为了构造测试替身再额外实现多个接口类型。

### 为什么不直接收敛为一个结构体？

相比接口，如果未来确实出现重复注入同一组依赖的场景，更适合考虑收敛为“依赖结构体”，例如：

```go
type LoadDeps struct {
	LocalLoad     LocalLoaderFunc
	RemoteLoad    RemoteLoaderFunc
	PayloadDecode PayloadDecodeFunc
}
```

然后改成：

```go
func LoadConfig[T any](params LoaderParams, deps LoadDeps) (T, error)
```

这种方式只是在“依赖集合”层面做收敛，不会把“参数”和“策略”塞进同一个对象。

### 什么时候再考虑收敛？

满足以下条件时，可以重新评估：

- 多个调用点总是在重复组装同一组 `LocalLoad / RemoteLoad / PayloadDecode`
- 调用层已经形成稳定的加载上下文对象
- 收敛后能明显减少重复代码，而不是仅仅让函数参数变少

### 术语边界

- `BootstrapConfig`：业务服务自己的启动配置模型，通常只初始化一次，不参与热更新
- `loader`：`go-micro/config` 中负责配置加载动作的通用能力
- `Raw.Encrypted`：表示当前整份配置内容是否为密文；读取时若为 `true`，则先解密整份内容，再解析目标结构

### 加密边界

- `go-micro/config` 只定义“是否加密”与“如何在读取时解码”的统一语义。
- 加密粒度是整份配置项，不做字段级加密。
- 如果只有局部敏感信息需要保护，应该把这部分内容拆成独立配置项，再决定该配置项是否整体加密。

这也是 `go-micro/config` 采用 `loader.go` 命名，而不直接定义业务侧 `BootstrapConfig` 的原因。
