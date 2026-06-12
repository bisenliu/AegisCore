package workerpool

import "sync/atomic"

// Stats 是任务池计数器的瞬时快照。
type Stats struct {
	Name    string
	Workers int

	Submitted int64
	Rejected  int64
	Started   int64
	Completed int64
	Failed    int64
	Panicked  int64
	Queued    int64
	Running   int64
	Closed    bool
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
