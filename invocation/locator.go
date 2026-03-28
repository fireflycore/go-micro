package invocation

import "context"

// StaticLocator 是最简单的 Locator 实现。
//
// 它不依赖任何外部注册中心，也不维护节点列表，
// 只负责按照统一规则把 ServiceRef 转换为标准 Target。
//
// 该实现非常适合作为：
// - `go-k8s/invocation` 的默认基础；
// - 单元测试中的轻量定位器；
// - 未来 etcd / consul 轻量实现的公共兜底逻辑。
type StaticLocator struct {
	// Options 控制 target 的构造方式，例如默认端口、集群域和 resolver scheme。
	Options TargetOptions
}

// Resolve 按照 StaticLocator 的固定规则，把 ServiceRef 转换为 Target。
func (s StaticLocator) Resolve(_ context.Context, ref ServiceRef) (Target, error) {
	return BuildTarget(ref, s.Options)
}
