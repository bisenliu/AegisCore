package logger

import (
	"errors"
	"syscall"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	"github.com/aegiscore/platform/runtime/config"
)

// NewLogger 提供配置化 zap logger，并在 Fx 关闭阶段执行同步。
func NewLogger(lc fx.Lifecycle, cfg *config.Config) (*zap.Logger, error) {
	log, err := New(cfg)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.StopHook(func() error {
		err := log.Sync()
		if isIgnorableSyncError(err) {
			// 某些平台的 stdout/stderr 不支持 fsync，关闭流程不应因此失败。
			return nil
		}
		return err
	}))
	return log, nil
}

// NewFxEventLogger 将 Fx 自身事件接入当前 App 的结构化 zap logger。
func NewFxEventLogger(log *zap.Logger) fxevent.Logger {
	fxLog := &fxevent.ZapLogger{Logger: log.Named("fx")}
	fxLog.UseLogLevel(zap.DebugLevel)
	fxLog.UseErrorLevel(zap.ErrorLevel)
	return fxLog
}

// isIgnorableSyncError 判断日志立即保存到磁盘时发生的错误是否可以安全忽略（例如在某些操作系统或控制台管道关闭时属于常见情况）。
func isIgnorableSyncError(err error) bool {
	return errors.Is(err, syscall.EBADF) || errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTTY)
}
