package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aegiscore/common/config"
	"go.uber.org/zap/zapcore"
)

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

func splitLogFilename(name string) (string, string) {
	stem := strings.TrimSuffix(name, ".log")
	if idx := strings.LastIndex(stem, "."); idx > 0 {
		return stem[:idx], stem[idx+1:]
	}
	return stem, "all"
}
