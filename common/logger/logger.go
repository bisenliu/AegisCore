package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aegiscore/common/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const TraceIDField = "trace-id"

type traceIDContextKey struct{}
type loggerContextKey struct{}

var defaultLogger = zap.NewNop()

func New(cfg *config.Config) (*zap.Logger, error) {
	logCfg := config.LogConfig{}
	if cfg != nil {
		logCfg = cfg.Log
	}
	return NewWithConfig(logCfg)
}

func NewWithConfig(cfg config.LogConfig) (*zap.Logger, error) {
	level := parseLevel(cfg.Level)
	encoder := newEncoder(cfg.Format)
	cores := make([]zapcore.Core, 0, 5)

	if cfg.Directory != "" || cfg.Filename != "" {
		writers, err := newFileWriters(cfg)
		if err != nil {
			return nil, err
		}
		cores = append(cores,
			zapcore.NewCore(encoder, writers.all, level),
			zapcore.NewCore(encoder, writers.info, exactLevelAtOrAbove(zapcore.InfoLevel, level)),
			zapcore.NewCore(encoder, writers.warning, exactLevelAtOrAbove(zapcore.WarnLevel, level)),
			zapcore.NewCore(encoder, writers.error, levelAtOrAbove(zapcore.ErrorLevel, level)),
		)
	}

	if cfg.Console || len(cores) == 0 {
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level))
	}

	log := zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	SetDefault(log)
	return log, nil
}

func SetDefault(log *zap.Logger) {
	if log == nil {
		defaultLogger = zap.NewNop()
		return
	}
	defaultLogger = log
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
		base = defaultLogger
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
	return WithContext(defaultLogger, ctx)
}

func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).Debug(msg, fields...)
}

func Info(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).Info(msg, fields...)
}

func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).Warn(msg, fields...)
}

func Error(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).Error(msg, fields...)
}

type fileWriters struct {
	all     zapcore.WriteSyncer
	info    zapcore.WriteSyncer
	warning zapcore.WriteSyncer
	error   zapcore.WriteSyncer
}

func newFileWriters(cfg config.LogConfig) (fileWriters, error) {
	dir := cfg.Directory
	if dir == "" {
		dir = "./logs"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fileWriters{}, fmt.Errorf("create log directory: %w", err)
	}
	filename := cfg.Filename
	if filename == "" {
		filename = "aegiscore"
	}
	return fileWriters{
		all:     newDailyLumberjackWriteSyncer(filepath.Join(dir, filename+".all.log"), cfg),
		info:    newDailyLumberjackWriteSyncer(filepath.Join(dir, filename+".info.log"), cfg),
		warning: newDailyLumberjackWriteSyncer(filepath.Join(dir, filename+".warning.log"), cfg),
		error:   newDailyLumberjackWriteSyncer(filepath.Join(dir, filename+".error.log"), cfg),
	}, nil
}

type dailyLumberjackWriteSyncer struct {
	mu       sync.Mutex
	filename string
	date     string
	newClock func() time.Time
	logger   *lumberjack.Logger
	cfg      config.LogConfig
}

func newDailyLumberjackWriteSyncer(filename string, cfg config.LogConfig) zapcore.WriteSyncer {
	w := &dailyLumberjackWriteSyncer{
		filename: filename,
		newClock: time.Now,
		cfg:      cfg,
	}
	w.rotateLocked()
	return zapcore.AddSync(w)
}

func (w *dailyLumberjackWriteSyncer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if currentDate := w.newClock().Format("2006-01-02"); currentDate != w.date {
		if err := w.rotateLocked(); err != nil {
			return 0, err
		}
	}
	return w.logger.Write(p)
}

func (w *dailyLumberjackWriteSyncer) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.logger == nil {
		return nil
	}
	return w.logger.Close()
}

func (w *dailyLumberjackWriteSyncer) rotateLocked() error {
	date := w.newClock().Format("2006-01-02")
	if w.logger != nil {
		if err := w.logger.Rotate(); err != nil {
			return err
		}
		if err := w.logger.Close(); err != nil {
			return err
		}
	}
	w.date = date
	w.logger = &lumberjack.Logger{
		Filename:   w.filename,
		MaxSize:    positiveOrDefault(w.cfg.MaxSizeMB, 100),
		MaxBackups: positiveOrDefault(w.cfg.MaxBackups, 30),
		MaxAge:     positiveOrDefault(w.cfg.MaxAgeDays, 7),
		LocalTime:  true,
		Compress:   false,
	}
	return nil
}

func newEncoder(format string) zapcore.Encoder {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.TimeKey = "time"
	cfg.LevelKey = "level"
	cfg.MessageKey = "msg"
	if strings.EqualFold(format, "console") || strings.EqualFold(format, "text") {
		return zapcore.NewConsoleEncoder(cfg)
	}
	return zapcore.NewJSONEncoder(cfg)
}

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
