package workerpool

// StatsSource 暴露任务池统计快照，供监控 adapter 只读消费。
type StatsSource interface {
	Stats() Stats
}
