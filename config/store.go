package config

import "context"

// Store 定义统一配置存储抽象，供 etcd/consul/k8s 实现对齐。
type Store interface {
	// Get 按配置键读取当前生效配置。
	Get(ctx context.Context, key Key) (*Raw, error)
	// GetByQuery 按运行时上下文读取配置。
	GetByQuery(ctx context.Context, query Query) (*Raw, error)
	// Put 写入当前生效配置。
	Put(ctx context.Context, key Key, item *Raw) error
	// Delete 删除当前配置。
	Delete(ctx context.Context, key Key) error

	// PutVersion 写入版本快照并返回版本号。
	PutVersion(ctx context.Context, key Key, item *Raw) (string, error)
	// GetVersion 读取指定版本快照。
	GetVersion(ctx context.Context, key Key, version string) (*Raw, error)
	// ListVersions 列出版本号列表。
	ListVersions(ctx context.Context, key Key, limit int) ([]string, error)

	// GetMeta 读取配置元信息。
	GetMeta(ctx context.Context, key Key) (*Meta, error)
	// PutMeta 写入配置元信息。
	PutMeta(ctx context.Context, key Key, meta *Meta) error
}
