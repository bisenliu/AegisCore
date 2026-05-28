package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/aegiscore/common/config"
)

func New(cfg *config.Config) *slog.Logger {
	return NewWithWriter(cfg.Log, os.Stdout)
}

func NewWithWriter(cfg config.LogConfig, w io.Writer) *slog.Logger {
	level := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{Level: level}
	if strings.EqualFold(cfg.Format, "text") {
		return slog.New(slog.NewTextHandler(w, opts))
	}
	return slog.New(slog.NewJSONHandler(w, opts))
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
