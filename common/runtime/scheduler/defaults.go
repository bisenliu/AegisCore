package scheduler

import "time"

const (
	// unlock 与 renew 使用独立的操作超时，避免任务 context 已取消后无法完成锁收尾。
	defaultLockUnlockTimeout = time.Second * 5
	defaultLockRenewTimeout  = time.Second * 5
)
