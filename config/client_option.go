package config

import "time"

const (
	// defaultClientCacheTTL 是启用本地缓存但未开启 watch 时的默认失效时间。
	defaultClientCacheTTL = 30 * time.Second
	// defaultClientCacheMaxEntries 是默认缓存容量上限。
	defaultClientCacheMaxEntries = 1024
)

// WatchMode 描述 Client 是否启用后台 watch 刷新缓存。
type WatchMode int

const (
	// WatchModeOff 表示关闭 watch，仅依赖直连读取或 TTL 缓存。
	WatchModeOff WatchMode = iota
	// WatchModeOn 表示开启 watch，由后台事件刷新本地缓存。
	WatchModeOn
)

// WatchScope 描述共享 watch 的聚合粒度。
type WatchScope int

const (
	// WatchScopePerKey 表示每个 key 维持独立监听。
	WatchScopePerKey WatchScope = iota
	// WatchScopeGroup 表示按 group 聚合监听。
	WatchScopeGroup
	// WatchScopeApp 表示按 app 聚合监听。
	WatchScopeApp
)

// ClientOptions 定义统一配置 Client 的运行参数。
type ClientOptions struct {
	// Timeout 控制单次底层读取的超时时间。
	Timeout time.Duration

	// EnableCache 控制是否启用本地缓存。
	EnableCache bool
	// CacheMaxEntries 控制缓存条目上限。
	CacheMaxEntries int
	// CacheTTL 控制缓存失效时间；通常用于未启用 watch 的场景。
	CacheTTL time.Duration

	// WatchMode 控制是否启用后台 watch。
	WatchMode WatchMode
	// WatchScope 控制共享 watch 的聚合粒度。
	WatchScope WatchScope
	// WatchBuffer 控制内部事件通道缓冲区大小。
	WatchBuffer int
}

// ClientOption 表示 Client 的函数式配置项。
type ClientOption func(*ClientOptions)

// NewClientOptions 生成带默认值的 ClientOptions，并按顺序应用外部参数。
func NewClientOptions(opts ...ClientOption) *ClientOptions {
	raw := &ClientOptions{
		Timeout:         5 * time.Second,
		EnableCache:     true,
		CacheMaxEntries: defaultClientCacheMaxEntries,
		CacheTTL:        defaultClientCacheTTL,
		WatchMode:       WatchModeOff,
		WatchScope:      WatchScopeGroup,
		WatchBuffer:     8,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(raw)
	}

	return raw
}

// WithClientTimeout 设置 Client 单次底层读取超时时间。
func WithClientTimeout(timeout time.Duration) ClientOption {
	return func(raw *ClientOptions) {
		if timeout <= 0 {
			return
		}
		raw.Timeout = timeout
	}
}

// WithClientCacheEnabled 设置是否启用本地缓存。
func WithClientCacheEnabled(enabled bool) ClientOption {
	return func(raw *ClientOptions) {
		raw.EnableCache = enabled
	}
}

// WithClientCacheMaxEntries 设置缓存条目上限。
func WithClientCacheMaxEntries(size int) ClientOption {
	return func(raw *ClientOptions) {
		if size <= 0 {
			return
		}
		raw.CacheMaxEntries = size
	}
}

// WithClientCacheTTL 设置缓存失效时间。
func WithClientCacheTTL(ttl time.Duration) ClientOption {
	return func(raw *ClientOptions) {
		if ttl <= 0 {
			return
		}
		raw.CacheTTL = ttl
	}
}

// WithClientWatchMode 设置后台 watch 开关。
func WithClientWatchMode(mode WatchMode) ClientOption {
	return func(raw *ClientOptions) {
		raw.WatchMode = mode
	}
}

// WithClientWatchScope 设置共享 watch 的聚合粒度。
func WithClientWatchScope(scope WatchScope) ClientOption {
	return func(raw *ClientOptions) {
		raw.WatchScope = scope
	}
}

// WithClientWatchBuffer 设置内部事件通道缓冲区大小。
func WithClientWatchBuffer(size int) ClientOption {
	return func(raw *ClientOptions) {
		if size <= 0 {
			return
		}
		raw.WatchBuffer = size
	}
}
