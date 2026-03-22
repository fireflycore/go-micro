package config

import "context"

// Watcher 定义统一配置监听抽象。
type Watcher interface {
	// Watch 监听指定配置键的变更事件。
	Watch(ctx context.Context, key Key) (<-chan WatchEvent, error)
	// Unwatch 取消指定配置键的监听。
	Unwatch(key Key)
}
