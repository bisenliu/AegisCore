package infrastructure

import (
	"log/slog"

	"github.com/aegiscore/common/config"
	"github.com/aegiscore/common/logger"
)

func NewLogger(cfg *config.Config) *slog.Logger {
	return logger.New(cfg)
}
