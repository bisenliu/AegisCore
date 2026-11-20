package bootstrap

import (
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/platform/runtime/logger"
)

// newRuntimeServerFailureHandler 在网络服务发生严重故障异常中断时记录日志，并通知系统安全停机退出。
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
