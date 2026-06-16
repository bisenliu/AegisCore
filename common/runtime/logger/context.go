package logger

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// TraceIDField 是请求关联 ID 使用的 zap 字段名。
const TraceIDField = "trace_id"

// SQLLoggerName 是 SQL 诊断日志使用的命名 logger。
const SQLLoggerName = "sql"

type traceIDContextKey struct{}
type loggerContextKey struct{}

var defaultLogger = struct {
	mu  sync.RWMutex
	log *zap.Logger
}{log: zap.NewNop()}

// SetDefault 替换进程级兜底 logger，并将 nil 视为 zap.NewNop。
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

// WithTraceID 返回携带 traceID 的 context，供 logger helper 使用。
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDContextKey{}, traceID)
}

// TraceIDFromContext 从 ctx 返回 trace ID，不存在时返回空字符串。
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if traceID, _ := ctx.Value(traceIDContextKey{}).(string); traceID != "" {
		return traceID
	}
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.TraceID().IsValid() {
		return ""
	}
	return spanContext.TraceID().String()
}

// WithContext 基于 base 派生 logger，并附加 ctx 中的 trace-id 字段。
func WithContext(ctx context.Context, base *zap.Logger) *zap.Logger {
	if base == nil {
		base = getDefault()
	}
	return base.With(zap.String(TraceIDField, TraceIDFromContext(ctx)))
}

// SQL 返回写入 SQL 专用日志流的命名 logger。
func SQL(base *zap.Logger) *zap.Logger {
	if base == nil {
		base = getDefault()
	}
	return base.Named(SQLLoggerName)
}

// ToContext 返回携带 log 的 context，供 FromContext 作为基础 logger 使用。
func ToContext(ctx context.Context, log *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, log)
}

// FromContext 返回 context logger 或默认 logger，并始终附加 trace-id。
func FromContext(ctx context.Context) *zap.Logger {
	if ctx != nil {
		if log, ok := ctx.Value(loggerContextKey{}).(*zap.Logger); ok && log != nil {
			return WithContext(ctx, log)
		}
	}
	return WithContext(ctx, getDefault())
}

// Debug 使用 context logger 写入 debug 日志，并保留调用方位置。
func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).WithOptions(zap.AddCallerSkip(1)).Debug(msg, fields...)
}

// Info 使用 context logger 写入 info 日志，并保留调用方位置。
func Info(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).WithOptions(zap.AddCallerSkip(1)).Info(msg, fields...)
}

// Warn 使用 context logger 写入 warning 日志，并保留调用方位置。
func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).WithOptions(zap.AddCallerSkip(1)).Warn(msg, fields...)
}

// Error 使用 context logger 写入 error 日志，并保留调用方位置。
func Error(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).WithOptions(zap.AddCallerSkip(1)).Error(msg, fields...)
}

// StackTrace 向 fields 追加 zap stacktrace 字段。
func StackTrace(fields ...zap.Field) []zap.Field {
	return append(fields, zap.Stack("stacktrace"))
}
