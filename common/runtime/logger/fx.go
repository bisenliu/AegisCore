package logger

import (
	"context"
	"syscall"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
)

// NewLogger 提供配置化 zap logger，并在 Fx 关闭阶段执行同步。
func NewLogger(lc fx.Lifecycle, cfg *config.Config) (*zap.Logger, error) {
	log, err := New(cfg)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{OnStop: func(context.Context) error {
		err := log.Sync()
		if err == syscall.EINVAL || err == syscall.ENOTTY {
			// 某些平台的 stdout/stderr 不支持 fsync，关闭流程不应因此失败。
			return nil
		}
		return err
	}})
	return log, nil
}
