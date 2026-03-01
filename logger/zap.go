package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New 构造一个 zap.Logger
//
// - Console=true 时输出到 stdout（面向人读）
// - Remote=true 且提供 handle 时输出 JSON 到 handle（面向机器解析）
// - 两者都未启用时返回 Nop logger，避免 nil 引用
func NewZapLogger(conf *Conf, handle func(b []byte)) *zap.Logger {
	// 允许传 nil：返回 nop，保持调用方简洁
	if conf == nil {
		return zap.NewNop()
	}

	// effectiveHandle 用于统一 handle 的来源：入参优先，其次使用 conf.handle
	effectiveHandle := handle
	if effectiveHandle == nil {
		effectiveHandle = conf.handle
	}
	// 统一将最终 handle 落到 conf 上，便于后续复用（例如外部读取配置对象的 handle）
	conf.handle = effectiveHandle

	// atomicLevel 作为 LevelEnabler 传递给各 core，使多个输出目的地共享同一等级门槛
	atomicLevel := zap.NewAtomicLevelAt(zap.InfoLevel)
	// 多个 core 通过 Tee 合并，保证同一条日志可同时输出到多个目的地
	cores := make([]zapcore.Core, 0, 2)
	if conf.Console {
		// Console 与 Remote 共用同一 LevelEnabler，避免两个输出目的地等级不一致
		cores = append(cores, NewConsoleCore(atomicLevel))
	}
	// Remote 需要 conf.handle，否则无法写入，避免产生“启用但无输出”的隐式失败
	if conf.Remote && effectiveHandle != nil {
		// Remote 走自定义 core：避免 JSON 编码后再解析/重组的额外开销
		cores = append(cores, NewRemoteCore(atomicLevel, effectiveHandle))
	}

	// 没有任何输出目的地时返回 nop，避免 NewTee 空参数造成不可预期行为
	if len(cores) == 0 {
		return zap.NewNop()
	}

	// AddCaller 会在日志中加入 caller 信息，字段名由 internal encoder 的 CallerKey 控制
	return zap.New(zapcore.NewTee(cores...), zap.AddCaller())
}
