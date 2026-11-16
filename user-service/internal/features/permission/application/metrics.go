package application

import (
	"context"
	"time"
)

//go:generate go run github.com/aegiscore/common/runtime/observability/metrics/nopgen -source metrics.go -type Metrics -output metrics_nop_gen.go -struct nopMetrics -func NopMetrics -comment "NopMetrics 返回 permission/RBAC 业务指标空实现。"

const (
	// MetricsOperationPolicyReload 表示 RBAC policy reload 操作。
	MetricsOperationPolicyReload = "policy_reload"
	// MetricsOperationPolicyPublish 表示 RBAC policy version 发布操作。
	MetricsOperationPolicyPublish = "policy_publish"
	// MetricsOperationWatcherReload 表示 watcher 触发的远端 reload 操作。
	MetricsOperationWatcherReload = "watcher_reload"
	// MetricsOperationWatcherRevisionCheck 表示 watcher 数据库 revision 补偿检查。
	MetricsOperationWatcherRevisionCheck = "watcher_revision_check"

	// MetricsReasonNone 表示操作成功或无需补充原因。
	MetricsReasonNone = "none"
	// MetricsReasonReloadFailed 表示 policy reload 失败。
	MetricsReasonReloadFailed = "reload_failed"
	// MetricsReasonPublishFailed 表示 policy version 发布失败。
	MetricsReasonPublishFailed = "publish_failed"
	// MetricsReasonRevisionStoreUnavailable 表示数据库 policy revision source 不可用。
	MetricsReasonRevisionStoreUnavailable = "revision_store_unavailable"
	// MetricsReasonRevisionMismatch 表示数据库 latest revision 超前于本地授权投影。
	MetricsReasonRevisionMismatch = "revision_mismatch"
	// MetricsReasonSystemError 表示未能稳定归类的系统异常。
	MetricsReasonSystemError = "system_error"
	// MetricsSourceLocalChange 表示本实例在线 RBAC 变更触发。
	MetricsSourceLocalChange = "local_change"
	// MetricsSourceWatcherPubSub 表示 watcher Pub/Sub 消息触发。
	MetricsSourceWatcherPubSub = "watcher_pubsub"
	// MetricsSourceWatcherRevisionCheck 表示 watcher 定时数据库 revision 补偿触发。
	MetricsSourceWatcherRevisionCheck = "watcher_revision_check"

	// MetricsEnforceResultAllow 表示授权判定通过。
	MetricsEnforceResultAllow = "allow"
	// MetricsEnforceResultDeny 表示授权判定拒绝。
	MetricsEnforceResultDeny = "deny"
	// MetricsEnforceResultError 表示授权判定异常。
	MetricsEnforceResultError = "error"
)

// Metrics 记录 permission/RBAC feature 的低基数业务指标。
type Metrics interface {
	PolicyReloadSucceeded(ctx context.Context, source string)
	PolicyReloadFailed(ctx context.Context, source string, reason string)
	PolicyPublishSucceeded(ctx context.Context)
	PolicyPublishFailed(ctx context.Context, reason string)
	WatcherCheckFailed(ctx context.Context, source string, reason string)
	WatcherReloadSucceeded(ctx context.Context, source string)
	WatcherReloadFailed(ctx context.Context, source string, reason string)
	WatcherVersionMismatch(ctx context.Context, source string, reason string)
	PolicyReloadLagObserved(ctx context.Context, lag int64)
	EnforceObserved(ctx context.Context, result string, method string, routeTemplate string, duration time.Duration)
}
