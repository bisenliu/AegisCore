package workerpool

import "sync/atomic"

// Stats 是任务池计数器的瞬时快照。
type Stats struct {
	// Name 是任务池名称。
	Name string
	// Workers 是任务池最大并发 worker 数。
	Workers int

	// Submitted 是已被 ants 接收的累计任务数。
	Submitted int64
	// Rejected 是提交阶段被拒绝的累计任务数。
	Rejected int64
	// Started 是已开始执行的累计任务数。
	Started int64
	// Completed 是成功完成的累计任务数。
	Completed int64
	// Failed 是执行返回错误的累计任务数。
	Failed int64
	// Panicked 是执行过程中 panic 并被恢复的累计任务数。
	Panicked int64
	// Queued 是已提交但尚未开始或尚未完成计数结转的任务数。
	Queued int64
	// Running 是当前正在执行的任务数。
	Running int64
	// Free 是 ants 当前空闲 worker 数。
	Free int64
	// Waiting 是正在等待空闲 worker 的提交方数量。
	Waiting int64
	// Closed 表示任务池是否已经停止接收新任务。
	Closed bool
}

type counters struct {
	submitted atomic.Int64
	rejected  atomic.Int64
	started   atomic.Int64
	completed atomic.Int64
	failed    atomic.Int64
	panicked  atomic.Int64
	queued    atomic.Int64
	running   atomic.Int64
}
