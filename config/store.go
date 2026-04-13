package config

import "context"

// Store 定义统一配置存储抽象，供 consul/k8s 实现对齐。
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

	// PutVersion 写入版本快照并返回版本号。
	// 若 Raw.Version 为空，实现层可自动生成版本号。
	PutVersion(ctx context.Context, key Key, raw *Raw) (string, error)

	// GetVersion 读取指定版本快照。
	GetVersion(ctx context.Context, key Key, version string) (*Raw, error)

	// ListVersions 列出版本号列表。
	// limit 为 0 时返回所有版本，否则返回最新的 limit 个版本。
	ListVersions(ctx context.Context, key Key, limit int) ([]string, error)

	// GetMeta 读取配置元信息。
	GetMeta(ctx context.Context, key Key) (*Meta, error)

	// PutMeta 写入配置元信息。
	PutMeta(ctx context.Context, key Key, meta *Meta) error
}
