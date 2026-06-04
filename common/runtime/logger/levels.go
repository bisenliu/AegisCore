package logger

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func parseLevel(level string) zapcore.Level {
	var parsed zapcore.Level
	if err := parsed.Set(strings.ToLower(level)); err != nil {
		parsed = zapcore.InfoLevel
	}
	return parsed
}

func exactLevelAtOrAbove(level zapcore.Level, minimum zapcore.Level) zap.LevelEnablerFunc {
	return func(got zapcore.Level) bool { return got == level && got >= minimum }
}

func levelAtOrAbove(level zapcore.Level, minimum zapcore.Level) zap.LevelEnablerFunc {
	return func(got zapcore.Level) bool { return got >= level && got >= minimum }
}

func positiveOrDefault(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
