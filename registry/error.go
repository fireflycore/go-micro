// Package registry 定义服务注册与发现的核心接口与通用模型。
package registry

import "errors"

var (
	// ErrRegisterIsNil 表示注册器为空。
	ErrRegisterIsNil = errors.New("注册器为空")
	// ErrServiceNodeNotExists 表示服务节点不存在。
	ErrServiceNodeNotExists = errors.New("服务节点不存在")
	// ErrServiceMethodNotExists 表示服务方法不存在。
	ErrServiceMethodNotExists = errors.New("服务方法不存在")
	// ErrServiceNodeMethodNotExists 表示服务节点不包含指定方法。
	ErrServiceNodeMethodNotExists = errors.New("服务节点不包含该方法")

	// ErrServiceConfigIsNil 表示服务配置为空。
	ErrServiceConfigIsNil = errors.New("服务配置为空")
	// ErrServiceNodeIsNil 表示服务节点对象为空。
	ErrServiceNodeIsNil = errors.New("服务节点对象为空")
	// ErrMetaAppIdIsEmpty 表示元数据 AppId 为空。
	ErrMetaAppIdIsEmpty = errors.New("元数据 AppId 为空")
	// ErrMetaEnvIsEmpty 表示元数据 Env 为空。
	ErrMetaEnvIsEmpty = errors.New("元数据 Env 为空")
)
