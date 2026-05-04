package logger

import (
	// context.Context 用来承载 trace、span 以及请求级附加信息。
	"context"

	// trace 包用于从 ctx 中提取当前 span 上下文。
	"go.opentelemetry.io/otel/trace"
	// zap 用于构造结构化日志字段。
	"go.uber.org/zap"
)

// appendContextFields 把 trace 相关字段和 otel bridge 所需的原始 ctx 注入日志字段。
func appendContextFields(ctx context.Context, logType string, fields []zap.Field) []zap.Field {
	// 预留额外容量，减少后续 append 时的切片扩容。
	result := make([]zap.Field, 0, len(fields)+4)
	// 先复制调用方已经传入的业务字段。
	result = append(result, fields...)

	// 只有在调用方还没传 log_type 时，才补默认日志类型。
	if logType != "" && !hasField(result, "log_type") {
		// 访问日志和服务日志分别会落成不同的 log_type。
		result = append(result, zap.String("log_type", logType))
	}

	// 从 ctx 中提取当前 span 的快照信息。
	spanCtx := trace.SpanContextFromContext(ctx)
	// 只有 span 有效时，trace_id/span_id 才有意义。
	if spanCtx.IsValid() {
		// 如果外部还没显式传 trace_id，就自动补一份可读字段。
		if !hasField(result, "trace_id") {
			// trace_id 便于在本地日志中直接检索整条链路。
			result = append(result, zap.String("trace_id", spanCtx.TraceID().String()))
		}
		// 如果外部还没显式传 span_id，就自动补一份可读字段。
		if !hasField(result, "span_id") {
			// span_id 便于精确定位当前操作对应的 span。
			result = append(result, zap.String("span_id", spanCtx.SpanID().String()))
		}
	}

	// 原始 ctx 仍然保留在字段里，供 otelzap core 在发 OTEL log record 时取用。
	// 普通 console core 会在外层被过滤掉这个字段，避免把整棵 context 树打印出来。
	result = append(result, zap.Any("ctx", ctx))

	// 返回补齐后的字段切片给上层 logger 使用。
	return result
}

// hasField 判断字段列表中是否已经存在指定 key。
func hasField(fields []zap.Field, key string) bool {
	// 逐个遍历字段做简单 key 匹配。
	for _, field := range fields {
		// 命中后直接返回 true，避免无意义继续扫描。
		if field.Key == key {
			return true
		}
	}
	// 扫描结束仍未命中，说明该 key 不存在。
	return false
}
