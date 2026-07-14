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

func lowLevelAtOrAbove(minimum zapcore.Level) zap.LevelEnablerFunc {
	return func(got zapcore.Level) bool { return got >= minimum && got < zapcore.WarnLevel }
}

func levelAtOrAbove(level zapcore.Level, minimum zapcore.Level) zap.LevelEnablerFunc {
	return func(got zapcore.Level) bool { return got >= level && got >= minimum }
}
