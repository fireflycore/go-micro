package consul

import "errors"

var (
	// ErrClientIsNil 表示 consul 客户端为空。
	ErrClientIsNil = errors.New("consul 客户端为空")
)
