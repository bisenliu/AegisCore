package application

import "time"

// PolicyWatcherSubscriptionState 表示 watcher 当前订阅阶段。
type PolicyWatcherSubscriptionState string

const (
	// PolicyWatcherSubscriptionStarting 表示 watcher 正在建立首次订阅。
	PolicyWatcherSubscriptionStarting PolicyWatcherSubscriptionState = "starting"
	// PolicyWatcherSubscriptionConnected 表示 watcher 已确认订阅。
	PolicyWatcherSubscriptionConnected PolicyWatcherSubscriptionState = "connected"
	// PolicyWatcherSubscriptionReconnecting 表示 watcher 正在退避并重建订阅。
	PolicyWatcherSubscriptionReconnecting PolicyWatcherSubscriptionState = "reconnecting"
	// PolicyWatcherSubscriptionStopped 表示 watcher 已停止。
	PolicyWatcherSubscriptionStopped PolicyWatcherSubscriptionState = "stopped"
)

// PolicyWatcherErrorCategory 表示 watcher 当前故障的低基数类别。
type PolicyWatcherErrorCategory string

const (
	// PolicyWatcherErrorNone 表示对应路径当前无故障。
	PolicyWatcherErrorNone PolicyWatcherErrorCategory = "none"
	// PolicyWatcherErrorSubscribe 表示订阅创建或确认失败。
	PolicyWatcherErrorSubscribe PolicyWatcherErrorCategory = "subscribe_failed"
	// PolicyWatcherErrorReceive 表示已确认订阅的接收路径失败。
	PolicyWatcherErrorReceive PolicyWatcherErrorCategory = "receive_failed"
	// PolicyWatcherErrorRevisionSource 表示数据库 revision 查询失败。
	PolicyWatcherErrorRevisionSource PolicyWatcherErrorCategory = "revision_store_unavailable"
	// PolicyWatcherErrorReload 表示本地 policy projection 未能达到数据库目标。
	PolicyWatcherErrorReload PolicyWatcherErrorCategory = "reload_failed"
)

// PolicyWatcherStatusSnapshot 是 RBAC policy watcher 的结构化只读状态快照。
type PolicyWatcherStatusSnapshot struct {
	Running                   bool
	SubscriptionState         PolicyWatcherSubscriptionState
	LastSubscriptionSuccessAt time.Time
	LastReconcileSuccessAt    time.Time
	LastFailureAt             time.Time
	SubscriptionErrorCategory PolicyWatcherErrorCategory
	ReconcileErrorCategory    PolicyWatcherErrorCategory
	ReconnectAttempts         uint64
}

// PolicyWatcherStatus 暴露 RBAC policy watcher 的只读运行状态。
type PolicyWatcherStatus interface {
	Status() PolicyWatcherStatusSnapshot
}
