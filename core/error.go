package micro

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
	// ErrRemoteResponseIsNil 表示远程调用返回的响应对象为空。
	ErrRemoteResponseIsNil = errors.New("远程响应为空")
	// ErrRemoteCallFailed 表示远程调用失败但未返回可读错误信息。
	ErrRemoteCallFailed = errors.New("远程调用失败")
)

// MetaKeyParseErrorFormat 用于构造元信息缺失/解析失败的错误文本。
const MetaKeyParseErrorFormat = "%s 解析失败"
