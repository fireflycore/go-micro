// Package registry 定义服务注册与发现的核心接口与通用模型。
package registry

import "errors"

var (
	// ErrClientIsNil 表示客户端为空
	ErrClientIsNil = "%s client is nil"

	// ErrRegisterIsNil 表示注册器为空。
	ErrRegisterIsNil = errors.New("注册器为空")
	// ErrServiceNodeNotExists 表示服务节点不存在。
	ErrServiceNodeNotExists = errors.New("服务节点不存在")
	// ErrServiceMethodNotExists 表示服务方法不存在。
	ErrServiceMethodNotExists = errors.New("服务方法不存在")
	// ErrServiceNodeMethodNotExists 表示服务节点不包含指定方法。
	ErrServiceNodeMethodNotExists = errors.New("服务节点不包含该方法")

	// ErrServiceConfIsNil 表示服务配置为空。
	ErrServiceConfIsNil = errors.New("service conf is nil")
	// ErrServiceMetaIsNil 标识服务元数据为空。
	ErrServiceMetaIsNil = errors.New("service meta is nil")
	// ErrServiceNodeIsNil 表示服务节点对象为空。
	ErrServiceNodeIsNil = errors.New("service node is nil")
)
