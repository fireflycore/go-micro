package config

import "context"

// Client 定义聚合 cache 与 watch 的统一配置读取入口。
// 当前阶段只先约束对外契约，具体后端实现由 consul/k8s 适配层在后续版本中补齐。
type Client interface {
	// Get 按配置键读取当前可用配置。
	// 调用方不感知底层是否命中缓存，或是否由 watch 已提前刷新。
	Get(ctx context.Context, key Key) (*Raw, error)
}
