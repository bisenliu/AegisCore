package scheduler

import "errors"

var (
	// ErrSchedulerStopped 表示调度器已停止接收新任务。
	ErrSchedulerStopped = errors.New("scheduler stopped")
	// ErrDuplicateJobKey 表示任务 key 已经注册。
	ErrDuplicateJobKey = errors.New("duplicate scheduler job key")
	// ErrInvalidJob 表示任务配置缺少必需字段或字段组合无效。
	ErrInvalidJob = errors.New("scheduler job is invalid")
	// ErrInvalidLock 表示分布式锁配置缺少必需字段或字段组合无效。
	ErrInvalidLock = errors.New("scheduler lock is invalid")
	// ErrLockNotOwned 表示当前实例不再持有目标锁。
	ErrLockNotOwned = errors.New("scheduler lock is not owned")
)
