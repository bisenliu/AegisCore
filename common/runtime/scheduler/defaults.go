package scheduler

import "time"

const (
	defaultLockUnlockTimeout = time.Second * 5
	defaultLockRenewTimeout  = time.Second * 5
)
