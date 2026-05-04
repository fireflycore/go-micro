package logger

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// TestAppendContextFieldsAddsTraceFieldsWithoutDuplicatingLogType 验证公共字段拼装逻辑。
func TestAppendContextFieldsAddsTraceFieldsWithoutDuplicatingLogType(t *testing.T) {
	// 构造一个有效的 span context，模拟真实链路场景。
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		// TraceID 用固定值，便于后续精确断言。
		TraceID: trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		// SpanID 同样使用固定值。
		SpanID: trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		// 标记该 span 为 sampled，更贴近真实运行状态。
		TraceFlags: trace.FlagsSampled,
		// 标记为远端 span，模拟由上游透传进来的链路。
		Remote: true,
	})
	// 把 span context 注入到 context 中，作为被测输入。
	ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)

	// 传入已有 log_type，验证函数不会重复追加同名字段。
	fields := appendContextFields(ctx, "access", []zap.Field{
		zap.String("log_type", "access"),
	})

	// counts 用来统计每个字段 key 出现的次数。
	counts := map[string]int{}
	// values 用来记录字符串类型字段的值。
	values := map[string]string{}
	// 扫描返回字段并收集断言所需信息。
	for _, field := range fields {
		// 统计该 key 的出现次数。
		counts[field.Key]++
		// 只提取字符串字段，便于后面比对 trace/span 值。
		if field.Type == zapcore.StringType {
			values[field.Key] = field.String
		}
	}

	// 确认 log_type 没有被重复追加。
	if counts["log_type"] != 1 {
		t.Fatalf("expected one log_type field, got %d", counts["log_type"])
	}
	// 确认 trace_id 已从 ctx 中正确提取。
	if values["trace_id"] != spanCtx.TraceID().String() {
		t.Fatalf("unexpected trace_id: %q", values["trace_id"])
	}
	// 确认 span_id 已从 ctx 中正确提取。
	if values["span_id"] != spanCtx.SpanID().String() {
		t.Fatalf("unexpected span_id: %q", values["span_id"])
	}
	// 确认原始 ctx 仍被保留给 otel core 使用。
	if counts["ctx"] != 1 {
		t.Fatalf("expected raw context field for otel core, got %d", counts["ctx"])
	}
}

// TestContextOmittingCoreDropsRawContextField 验证普通输出 core 会过滤掉原始 ctx。
func TestContextOmittingCoreDropsRawContextField(t *testing.T) {
	// 用 observer 构造一个可观测的内存 core。
	baseCore, observed := observer.New(zapcore.InfoLevel)
	// 给内存 core 套上 context 过滤包装。
	logger := zap.New(NewContextOmittingCore(baseCore))

	// 写入一条同时包含普通字段和原始 ctx 的日志。
	logger.Info("hello",
		zap.String("k", "v"),
		zap.Any("ctx", context.Background()),
	)

	// 读取捕获到的全部日志条目。
	entries := observed.All()
	// 预期只收到一条日志。
	if len(entries) != 1 {
		t.Fatalf("expected one log entry, got %d", len(entries))
	}
	// 断言 ctx 字段已经被过滤掉，不会进入普通输出。
	if got := entries[0].ContextMap()["ctx"]; got != nil {
		t.Fatalf("expected ctx field to be dropped, got %#v", got)
	}
	// 断言普通业务字段仍然完整保留。
	if got := entries[0].ContextMap()["k"]; got != "v" {
		t.Fatalf("expected k field to be preserved, got %#v", got)
	}
}
