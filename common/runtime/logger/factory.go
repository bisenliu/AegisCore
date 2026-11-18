package logger

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/aegiscore/common/runtime/config"
)

// New 根据根配置创建调用方拥有的 zap logger，并在配置为 nil 时使用默认配置。
func New(cfg *config.Config) (*zap.Logger, error) {
	logCfg := config.LogConfig{}
	serviceName := ""
	environment := ""
	if cfg != nil {
		logCfg = cfg.Log
		serviceName = strings.TrimSpace(cfg.App.Name)
		environment = strings.TrimSpace(cfg.App.Environment)
	}
	return newLogger(logCfg, serviceName, environment, zapcore.AddSync(os.Stdout), zapcore.AddSync(os.Stderr))
}

// NewWithConfig 创建调用方拥有的配置化 zap logger。
func NewWithConfig(cfg config.LogConfig) (*zap.Logger, error) {
	return newLogger(cfg, "", "", zapcore.AddSync(os.Stdout), zapcore.AddSync(os.Stderr))
}

// newLogger 根据配置创建日志输出工具，支持区分输出级别（如普通信息或警告）并分别导向控制台或文件。
func newLogger(cfg config.LogConfig, serviceName string, environment string, stdout zapcore.WriteSyncer, stderr zapcore.WriteSyncer) (*zap.Logger, error) {
	level := parseLevel(cfg.Level)
	core := zapcore.NewTee(
		zapcore.NewCore(newEncoder(cfg.Format), stdout, lowLevelAtOrAbove(level)),
		zapcore.NewCore(newEncoder(cfg.Format), stderr, levelAtOrAbove(zapcore.WarnLevel, level)),
	)
	log := zap.New(core, zap.AddCaller()).Named(ApplicationLoggerName)
	identityFields := make([]zap.Field, 0, 2)
	if serviceName != "" {
		identityFields = append(identityFields, zap.String(ServiceField, serviceName))
	}
	if environment != "" {
		identityFields = append(identityFields, zap.String(EnvironmentField, environment))
	}
	log = log.With(identityFields...)
	return log, nil
}

// newEncoder 根据配置选择日志输出格式，例如适合机器自动收集的结构化 JSON 格式，或是适合人工肉眼阅读的彩色控制台文本。
func newEncoder(format string) zapcore.Encoder {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.TimeKey = "time"
	cfg.LevelKey = "level"
	cfg.MessageKey = "msg"
	if strings.EqualFold(format, "console") {
		return zapcore.NewConsoleEncoder(cfg)
	}
	return zapcore.NewJSONEncoder(cfg)
}
