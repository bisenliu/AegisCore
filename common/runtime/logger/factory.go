package logger

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/aegiscore/common/runtime/config"
)

// New 根据根配置创建 zap logger，并在配置为 nil 时使用默认配置。
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

// NewWithConfig 创建配置化 zap logger，并将其安装为进程默认 logger。
func NewWithConfig(cfg config.LogConfig) (*zap.Logger, error) {
	return newLogger(cfg, "", "", zapcore.AddSync(os.Stdout), zapcore.AddSync(os.Stderr))
}

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
	SetDefault(log)
	return log, nil
}

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
