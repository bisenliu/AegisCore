package logger

import (
	"os"
	"strings"

	"github.com/aegiscore/common/runtime/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New 根据根配置创建 zap logger，并在配置为 nil 时使用默认配置。
func New(cfg *config.Config) (*zap.Logger, error) {
	logCfg := config.LogConfig{}
	if cfg != nil {
		logCfg = cfg.Log
	}
	return NewWithConfig(logCfg)
}

// NewWithConfig 创建配置化 zap logger，并将其安装为进程默认 logger。
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
			// 文件日志同时写入聚合日志和按级别拆分的运维日志流。
			zapcore.NewCore(encoder, writers.all, level),
			zapcore.NewCore(encoder, writers.info, exactLevelAtOrAbove(zapcore.InfoLevel, level)),
			zapcore.NewCore(encoder, writers.warning, exactLevelAtOrAbove(zapcore.WarnLevel, level)),
			zapcore.NewCore(encoder, writers.error, levelAtOrAbove(zapcore.ErrorLevel, level)),
		)
	}

	if cfg.Console || len(cores) == 0 {
		// 未配置文件输出时保持 stdout 日志可用，即使 Console 未显式开启。
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level))
	}

	log := zap.New(zapcore.NewTee(cores...), zap.AddCaller())
	SetDefault(log)
	return log, nil
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
