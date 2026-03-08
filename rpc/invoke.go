package rpc

import (
	"errors"
	"reflect"
)

// RemoteResponse 定义远程调用响应的标准接口。
type RemoteResponse[T any] interface {
	GetCode() uint32    // 获取状态码
	GetMessage() string // 获取消息文本
	GetData() T         // 获取业务数据
}

// invokeRemote 是内部核心实现，返回业务数据、状态码和错误。
func invokeRemote[T any, R RemoteResponse[T]](callFunc func() (R, error)) (data T, code uint32, err error) {
	var zero T

	// 网络/框架层错误
	resp, err := callFunc()
	if err != nil {
		return zero, 500, err
	}

	// 防御“带类型的 nil”
	respValue := reflect.ValueOf(resp)
	if respValue.Kind() == reflect.Ptr && respValue.IsNil() {
		return zero, 500, ErrRemoteResponseIsNil
	}

	code = resp.GetCode()
	if code != 200 {
		msg := resp.GetMessage()
		if msg == "" {
			return zero, code, ErrRemoteCallFailed
		}
		return zero, code, errors.New(msg)
	}

	return resp.GetData(), code, nil
}

// InvokeRemote 执行远程调用并处理标准化响应，仅返回业务数据或错误。
// T: 业务数据类型
// R: 响应类型，必须实现 RemoteResponse[T] 接口
func InvokeRemote[T any, R RemoteResponse[T]](callFunc func() (R, error)) (T, error) {
	data, _, err := invokeRemote(callFunc)
	return data, err
}

// InvokeRemoteWithCode 执行远程调用并处理标准化响应，返回业务数据、状态码和错误。
// T: 业务数据类型
// R: 响应类型，必须实现 RemoteResponse[T] 接口
func InvokeRemoteWithCode[T any, R RemoteResponse[T]](callFunc func() (R, error)) (T, uint32, error) {
	return invokeRemote(callFunc)
}
