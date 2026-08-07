package bootstrap

import (
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
)

func newRuntimeServerFailureHandler(log *zap.Logger, shutdowner fx.Shutdowner, serverName string) func(error) {
	return func(err error) {
		log.Error(fmt.Sprintf("%s server failed", serverName), logger.StackTrace(zap.Error(err))...)
		if shutdowner == nil {
			return
		}
		if shutdownErr := shutdowner.Shutdown(fx.ExitCode(1)); shutdownErr != nil {
			log.Error(
				fmt.Sprintf("shutdown after %s server failure failed", serverName),
				logger.StackTrace(zap.Error(shutdownErr))...,
			)
		}
	}
}
