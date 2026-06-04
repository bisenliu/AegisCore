package loggerfx

import (
	"context"
	"syscall"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func NewLogger(lc fx.Lifecycle, cfg *config.Config) (*zap.Logger, error) {
	log, err := logger.New(cfg)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{OnStop: func(context.Context) error {
		err := log.Sync()
		if err == syscall.EINVAL || err == syscall.ENOTTY {
			return nil
		}
		return err
	}})
	return log, nil
}
