// Package rpc 提供标准化的 RPC 调用封装与响应处理工具。
package rpc

import "errors"

var (
	// ErrRemoteResponseIsNil 表示远程调用返回的响应对象为空。
	ErrRemoteResponseIsNil = errors.New("远程响应为空")
	// ErrRemoteCallFailed 表示远程调用失败但未返回可读错误信息。
	ErrRemoteCallFailed = errors.New("远程调用失败")
)

// MetaKeyParseErrorFormat 用于构造元信息缺失/解析失败的错误文本。
const MetaKeyParseErrorFormat = "%s 解析失败"
