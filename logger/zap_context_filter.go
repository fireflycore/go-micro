package logger

import (
	"context"

	"go.uber.org/zap/zapcore"
)

// ContextOmittingCore 包装普通 zap core，专门过滤掉原始 context 字段。
type ContextOmittingCore struct {
	core zapcore.Core
}

// NewContextOmittingCore 为普通 core 套上一层上下文字段过滤。
func NewContextOmittingCore(core zapcore.Core) zapcore.Core {
	// 兼容空 core，避免上层额外判空。
	if core == nil {
		return nil
	}
	// 返回带过滤能力的包装 core。
	return ContextOmittingCore{core: core}
}

// Enabled 直接复用底层 core 的等级判断逻辑。
func (c ContextOmittingCore) Enabled(level zapcore.Level) bool {
	// 是否允许写日志完全交给底层 core 决定。
	return c.core.Enabled(level)
}

// With 在继承字段时先去掉 context 字段，避免它进入普通输出链路。
func (c ContextOmittingCore) With(fields []zapcore.Field) zapcore.Core {
	// 先过滤字段，再把结果继续交给底层 core 派生。
	return ContextOmittingCore{core: c.core.With(filterContextFields(fields))}
}

// Check 决定当前 entry 是否应由该 core 参与写出。
func (c ContextOmittingCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	// 如果等级未启用，直接返回已有 checked entry。
	if !c.Enabled(ent.Level) {
		return ce
	}
	// 等级启用时，把当前包装 core 注册到 checked entry 中。
	return ce.AddCore(ent, c)
}

// Write 在真正写日志前再次过滤原始 context 字段。
func (c ContextOmittingCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	// 过滤后的字段交由底层 core 正常编码输出。
	return c.core.Write(ent, filterContextFields(fields))
}

// Sync 把 flush 动作透传给底层 core。
func (c ContextOmittingCore) Sync() error {
	// 保持与底层 core 相同的刷新行为。
	return c.core.Sync()
}

// filterContextFields 从字段列表中移除所有 context.Context 类型的字段。
func filterContextFields(fields []zapcore.Field) []zapcore.Field {
	// 空切片直接返回，避免无意义分配。
	if len(fields) == 0 {
		return fields
	}

	// 预分配与原始长度相同的容量，减少追加时的扩容。
	filtered := make([]zapcore.Field, 0, len(fields))
	// 顺序扫描所有字段。
	for _, field := range fields {
		// 只要字段底层值实现了 context.Context，就把它过滤掉。
		if _, ok := field.Interface.(context.Context); ok {
			continue
		}
		// 其他普通字段原样保留。
		filtered = append(filtered, field)
	}

	// 返回去掉 ctx 后的字段结果。
	return filtered
}
