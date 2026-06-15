package scheduler

import (
	"fmt"

	"go.uber.org/zap"
)

type cronZapLogger struct {
	logger *zap.Logger
}

func (l cronZapLogger) Info(msg string, keysAndValues ...any) {
	l.logger.Info(msg, zapFields(keysAndValues)...)
}

func (l cronZapLogger) Error(err error, msg string, keysAndValues ...any) {
	fields := append(zapFields(keysAndValues), zap.Error(err))
	l.logger.Error(msg, fields...)
}

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
