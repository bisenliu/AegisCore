package scheduler

import "time"

const (
	// defaultLockUnlockTimeout 是任务完成后释放分布式锁的操作超时时间。
	defaultLockUnlockTimeout = time.Second * 5
	// defaultLockRenewTimeout 是周期性延长分布式锁租期的操作超时时间。
	defaultLockRenewTimeout = time.Second * 5
	// defaultLockRetryInitialInterval 是获取分布式锁时的默认初始重试等待间隔。
	defaultLockRetryInitialInterval = 50 * time.Millisecond
	// defaultLockRetryMaxInterval 是获取分布式锁时的默认最大重试等待间隔。
	defaultLockRetryMaxInterval = time.Second
	// lockRandomTokenBytes 是生成分布式锁持有者唯一 Token 所需的随机安全字节数（16 字节 = 128 位）。
	lockRandomTokenBytes = 16
)
