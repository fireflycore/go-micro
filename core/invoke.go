package micro

import (
	"errors"
	"reflect"
)

// RemoteResponse 定义远程调用响应的标准接口
type RemoteResponse[T any] interface {
	GetCode() uint32    // 获取状态码
	GetMessage() string // 获取消息文本
	GetData() T         // 获取业务数据
}

// WithRemoteInvoke 执行远程调用并处理标准化响应
// T: 业务数据类型
// R: 响应类型，必须实现 RemoteResponse[T] 接口
func WithRemoteInvoke[T any, R RemoteResponse[T]](callFunc func() (R, error)) (T, error) {
	var zero T

	// 1. 先透传网络/框架层错误：这类错误通常包含重试/降级所需的信息。
	resp, err := callFunc()
	if err != nil {
		return zero, err
	}

	// 2. 防御“带类型的 nil”：
	// 在泛型 + interface 约束下，resp 可能是 *T(nil) 但接口值不为 nil，直接使用会 panic。
	respValue := reflect.ValueOf(resp)
	if respValue.Kind() == reflect.Ptr && respValue.IsNil() {
		return zero, ErrRemoteResponseIsNil
	}

	// 3. 统一以 code=200 表示成功，非 200 视为业务失败并优先返回服务端 message。
	if code := resp.GetCode(); code != 200 {
		msg := resp.GetMessage()
		if msg == "" {
			return zero, ErrRemoteCallFailed
		}
		return zero, errors.New(msg)
	}

	// 4. 成功时仅返回业务数据，调用方不需要关心响应封装结构。
	data := resp.GetData()

	return data, nil
}
