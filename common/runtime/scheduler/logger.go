package scheduler

import (
	"fmt"

	"go.uber.org/zap"
)

// cronZapLogger 将 robfig/cron 的键值日志适配为 scheduler 注入的 Zap logger。
type cronZapLogger struct {
	logger *zap.Logger
}

// Info 把 cron info 级键值日志写入 Zap。
func (l cronZapLogger) Info(msg string, keysAndValues ...any) {
	l.logger.Info(msg, zapFields(keysAndValues)...)
}

// Error 把 cron error 和附加键值字段写入 Zap。
func (l cronZapLogger) Error(err error, msg string, keysAndValues ...any) {
	fields := append(zapFields(keysAndValues), zap.Error(err))
	l.logger.Error(msg, fields...)
}

// zapFields 容忍 cron 传入奇数个键值，避免日志适配器本身触发 panic。
func zapFields(keysAndValues []any) []zap.Field {
	fields := make([]zap.Field, 0, len(keysAndValues)/2)
	for index := 0; index < len(keysAndValues); index += 2 {
		key := fmt.Sprint(keysAndValues[index])
		if index+1 >= len(keysAndValues) {
			fields = append(fields, zap.Any(key, nil))
			break
		}
		fields = append(fields, zap.Any(key, keysAndValues[index+1]))
	}
	return fields
}
