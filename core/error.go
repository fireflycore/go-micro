package micro

import "errors"

var (
	// 注册器为空
	ErrRegisterIsNil = errors.New("注册器为空")
	// 服务节点不存在
	ErrServiceNodeNotExists = errors.New("服务节点不存在")
	// 服务方法不存在
	ErrServiceMethodNotExists = errors.New("服务方法不存在")
	// 服务节点不包含该方法
	ErrServiceNodeMethodNotExists = errors.New("服务节点不包含该方法")
	// 远程响应为空
	ErrRemoteResponseIsNil = errors.New("远程响应为空")
	// 远程调用失败
	ErrRemoteCallFailed = errors.New("远程调用失败")
)

const MetaKeyParseErrorFormat = "%s 解析失败"
