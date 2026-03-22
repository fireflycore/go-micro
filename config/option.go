package config

import "time"

// Codec 定义配置内容序列化与反序列化能力。
type Codec interface {
	// Marshal 把结构化对象编码为字节。
	Marshal(v any) ([]byte, error)
	// Unmarshal 把字节解码到目标对象。
	Unmarshal(data []byte, dst any) error
}

// Encryptor 定义配置内容加解密能力。
type Encryptor interface {
	// Encrypt 使用给定密钥加密数据。
	Encrypt(data []byte, key []byte) ([]byte, error)
	// Decrypt 使用给定密钥解密数据。
	Decrypt(data []byte, key []byte) ([]byte, error)
}

// Options 定义 config 组件的通用运行参数。
type Options struct {
	// Namespace 用于区分配置键空间。
	Namespace string
	// Timeout 用于单次请求超时控制。
	Timeout time.Duration
	// Retry 用于失败重试次数控制。
	Retry uint32
	// WatchBuffer 用于监听事件通道缓冲区大小。
	WatchBuffer int
	// Codec 为可选编解码器。
	Codec Codec
	// Encryptor 为可选加解密器。
	Encryptor Encryptor
}

// Option 表示函数式配置项。
type Option func(*Options)

// NewOptions 生成带默认值的 Options，并按顺序应用外部参数。
func NewOptions(opts ...Option) *Options {
	// 初始化默认配置，保证零参数也可运行。
	raw := &Options{
		Timeout:     5 * time.Second,
		Retry:       3,
		WatchBuffer: 8,
	}
	// 逐个应用调用方传入的 Option。
	for _, opt := range opts {
		// 跳过空函数，避免 panic。
		if opt == nil {
			continue
		}
		// 应用当前 Option。
		opt(raw)
	}
	// 返回最终配置。
	return raw
}

// WithNamespace 设置配置命名空间。
func WithNamespace(namespace string) Option {
	return func(raw *Options) {
		raw.Namespace = namespace
	}
}

// WithTimeout 设置单次请求超时时间。
func WithTimeout(timeout time.Duration) Option {
	return func(raw *Options) {
		// 非正数不生效，避免错误配置覆盖默认值。
		if timeout <= 0 {
			return
		}
		raw.Timeout = timeout
	}
}

// WithRetry 设置失败重试次数。
func WithRetry(retry uint32) Option {
	return func(raw *Options) {
		// 0 表示不修改默认值。
		if retry == 0 {
			return
		}
		raw.Retry = retry
	}
}

// WithWatchBuffer 设置监听事件缓冲区大小。
func WithWatchBuffer(size int) Option {
	return func(raw *Options) {
		// 非正数不生效，避免无效缓冲区。
		if size <= 0 {
			return
		}
		raw.WatchBuffer = size
	}
}

// WithCodec 注入自定义编解码实现。
func WithCodec(codec Codec) Option {
	return func(raw *Options) {
		raw.Codec = codec
	}
}

// WithEncryptor 注入自定义加解密实现。
func WithEncryptor(encryptor Encryptor) Option {
	return func(raw *Options) {
		raw.Encryptor = encryptor
	}
}
