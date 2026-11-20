package logger

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// parseLevel 将文本形式的日志级别（如 info、debug、error）转换为系统标准级别；如果识别不出则默认使用普通级别（info）。
func parseLevel(level string) zapcore.Level {
	var parsed zapcore.Level
	if err := parsed.Set(strings.ToLower(level)); err != nil {
		parsed = zapcore.InfoLevel
	}
	return parsed
}

// lowLevelAtOrAbove 过滤出普通信息级别的日志，允许常规业务日志输出到标准输出，而将更高级别的警告和错误留给专门的通道。
func lowLevelAtOrAbove(minimum zapcore.Level) zap.LevelEnablerFunc {
	return func(got zapcore.Level) bool { return got >= minimum && got < zapcore.WarnLevel }
}

// levelAtOrAbove 检查某条日志的重要性是否达到了系统允许输出的门槛，过滤掉过于啰嗦的低优先级调试日志。
func levelAtOrAbove(level zapcore.Level, minimum zapcore.Level) zap.LevelEnablerFunc {
	return func(got zapcore.Level) bool { return got >= level && got >= minimum }
}
