package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap/zapcore"

	"github.com/aegiscore/common/runtime/config"
)

type fileWriters struct {
	all     zapcore.WriteSyncer
	info    zapcore.WriteSyncer
	warning zapcore.WriteSyncer
	error   zapcore.WriteSyncer
	sql     zapcore.WriteSyncer
}

func newFileWriters(cfg config.LogConfig) (fileWriters, error) {
	dir := cfg.Directory
	if dir == "" {
		dir = "./logs"
	}
	// 0755 允许服务用户写入日志，同时允许运维人员查看日志目录。
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
		sql:     newDailyLumberjackWriteSyncer(filepath.Join(dir, filename+".sql.log"), cfg),
	}, nil
}

func splitLogFilename(name string) (string, string) {
	stem := strings.TrimSuffix(name, ".log")
	if idx := strings.LastIndex(stem, "."); idx > 0 {
		return stem[:idx], stem[idx+1:]
	}
	return stem, "all"
}
