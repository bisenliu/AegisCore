package logger

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

const TraceIDField = "trace-id"

type traceIDContextKey struct{}
type loggerContextKey struct{}

var defaultLogger = struct {
	mu  sync.RWMutex
	log *zap.Logger
}{log: zap.NewNop()}

func SetDefault(log *zap.Logger) {
	if log == nil {
		log = zap.NewNop()
	}
	defaultLogger.mu.Lock()
	defaultLogger.log = log
	defaultLogger.mu.Unlock()
}

func getDefault() *zap.Logger {
	defaultLogger.mu.RLock()
	defer defaultLogger.mu.RUnlock()
	return defaultLogger.log
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDContextKey{}, traceID)
}

func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	traceID, _ := ctx.Value(traceIDContextKey{}).(string)
	return traceID
}

func WithContext(base *zap.Logger, ctx context.Context) *zap.Logger {
	if base == nil {
		base = getDefault()
	}
	return base.With(zap.String(TraceIDField, TraceIDFromContext(ctx)))
}

func ToContext(ctx context.Context, log *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, log)
}

func FromContext(ctx context.Context) *zap.Logger {
	if ctx != nil {
		if log, ok := ctx.Value(loggerContextKey{}).(*zap.Logger); ok && log != nil {
			return WithContext(log, ctx)
		}
	}
	return WithContext(getDefault(), ctx)
}

func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).WithOptions(zap.AddCallerSkip(1)).Debug(msg, fields...)
}

func Info(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).WithOptions(zap.AddCallerSkip(1)).Info(msg, fields...)
}

func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).WithOptions(zap.AddCallerSkip(1)).Warn(msg, fields...)
}

func Error(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).WithOptions(zap.AddCallerSkip(1)).Error(msg, fields...)
}

func StackTrace(fields ...zap.Field) []zap.Field {
	return append(fields, zap.Stack("stacktrace"))
}
