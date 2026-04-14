package config

import "context"

// Store 定义统一配置存储抽象，供 consul/k8s 实现对齐。
// 配置变更监听能力由独立的 Watcher 接口承载，不并入 Store。
type Store interface {
	// Get 按配置键读取当前生效配置。
	// 读取到的 Raw.Encrypted 标识整份配置是否为密文。
	// 当 Encrypted=true 时，调用方需要先解密整份 Content，再解析目标结构。
	Get(ctx context.Context, key Key) (*Raw, error)

	// Put 写入当前生效配置。
	// Raw.Encrypted 标识整份配置是否为密文。
	// 一份配置要加密就整份加密，不做字段级加密。
	Put(ctx context.Context, key Key, raw *Raw) error

	// Delete 删除当前配置。
	Delete(ctx context.Context, key Key) error
}
