// Package invocation 定义面向 service-to-service 调用模型的核心错误。
package invocation

import "errors"

var (
	// ErrServiceNameEmpty 表示服务名为空，无法构造逻辑服务身份。
	ErrServiceNameEmpty = errors.New("service name is empty")
	// ErrNamespaceEmpty 表示命名空间为空。
	ErrNamespaceEmpty = errors.New("namespace is empty")
	// ErrTargetHostEmpty 表示目标主机为空，无法生成最终拨号地址。
	ErrTargetHostEmpty = errors.New("target host is empty")
	// ErrTargetPortInvalid 表示端口既未显式提供，也无法从默认值中补齐。
	ErrTargetPortInvalid = errors.New("target port is invalid")
	// ErrDNSManagerIsNil 表示 DNS 管理器为空。
	ErrDNSManagerIsNil = errors.New("dns manager is nil")
	// ErrDialFnIsNil 表示底层拨号函数为空。
	ErrDialFnIsNil = errors.New("dial function is nil")
	// ErrConnectionManagerClosed 表示连接管理器已经关闭，不能再创建新连接。
	ErrConnectionManagerClosed = errors.New("connection manager is closed")
	// ErrInvokerDialerIsNil 表示调用器缺少拨号器依赖。
	ErrInvokerDialerIsNil = errors.New("invoker dialer is nil")
	// ErrInvokeMethodEmpty 表示调用方法名为空。
	ErrInvokeMethodEmpty = errors.New("invoke method is empty")
	// ErrRemoteServiceNotFound 表示未找到指定远程业务服务。
	ErrRemoteServiceNotFound = errors.New("remote service not found")
)
