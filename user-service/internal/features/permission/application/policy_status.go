package application

// PolicyWatcherStatus 暴露 RBAC policy watcher 的只读运行状态。
type PolicyWatcherStatus interface {
	Running() bool
	LastError() error
}
