package config

import "errors"

var (
	// ErrStoreIsNil 表示存储实现为空。
	ErrStoreIsNil = errors.New("config store is nil")
	// ErrWatcherIsNil 表示监听实现为空。
	ErrWatcherIsNil = errors.New("config watcher is nil")
	// ErrCodecIsNil 表示编解码实现为空。
	ErrCodecIsNil = errors.New("config codec is nil")
	// ErrEncryptorIsNil 表示加解密实现为空。
	ErrEncryptorIsNil = errors.New("config encryptor is nil")
	// ErrInvalidKey 表示配置键不合法。
	ErrInvalidKey = errors.New("invalid config key")
	// ErrInvalidItem 表示配置内容不合法。
	ErrInvalidItem = errors.New("invalid config item")
	// ErrResourceNotFound 表示配置资源不存在。
	ErrResourceNotFound = errors.New("config resource not found")
	// ErrVersionConflict 表示写入版本冲突。
	ErrVersionConflict = errors.New("config version conflict")
)
