package etcd

import "errors"

var (
	// ErrClientIsNil 表示 etcd 客户端为空。
	ErrClientIsNil = errors.New("etcd 客户端为空")
)
