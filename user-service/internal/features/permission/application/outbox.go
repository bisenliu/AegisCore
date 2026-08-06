package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	OutboxStatusPending    = "pending"
	OutboxStatusProcessing = "processing"
	OutboxStatusFailed     = "failed"
	OutboxStatusDelivered  = "delivered"

	DispatcherErrorNone           = ""
	DispatcherErrorClaim          = "claim_failed"
	DispatcherErrorPublish        = "publish_failed"
	DispatcherErrorAck            = "ack_failed"
	DispatcherErrorFailureRecord  = "failure_record_failed"
	DispatcherErrorClaimLost      = "claim_lost"
	DispatcherErrorBacklog        = "backlog_failed"
	DispatcherErrorUnexpectedExit = "unexpected_exit"
)

// ErrOutboxClaimLost 表示事件的 lease 已被其他 dispatcher 接管，当前 worker 不得再更新该事件。
var ErrOutboxClaimLost = errors.New("rbac policy outbox claim lost")

// OutboxEvent 是 dispatcher 发布所需的最小 RBAC policy 事件。
type OutboxEvent struct {
	EventID        uuid.UUID
	Revision       int64
	Kind           string
	Reason         string
	RoleID         *uuid.UUID
	UserID         *uuid.UUID
	PermissionID   *uuid.UUID
	IdempotencyKey string
}

// OutboxClaim 表示由 PostgreSQL lease 仲裁后的单次处理权。
type OutboxClaim struct {
	Event        OutboxEvent
	ClaimToken   uuid.UUID
	AttemptCount int
}

// OutboxBacklog 是未完成事件的只读快照。
type OutboxBacklog struct {
	DueCount        int
	OldestCreatedAt *time.Time
}

// OutboxStore 定义 dispatcher 消费的持久化状态机端口。
type OutboxStore interface {
	Claim(ctx context.Context, now time.Time, limit int, claimTimeout time.Duration) ([]OutboxClaim, error)
	Ack(ctx context.Context, eventID uuid.UUID, claimToken uuid.UUID, deliveredAt time.Time) (bool, error)
	Fail(ctx context.Context, eventID uuid.UUID, claimToken uuid.UUID, failedAt time.Time, nextAttemptAt time.Time, errorSummary string) (bool, error)
	Backlog(ctx context.Context, now time.Time) (OutboxBacklog, error)
}

// PolicyRevisionPublisher 发布已持久化的 RBAC policy revision 通知。
type PolicyRevisionPublisher interface {
	PublishPolicyRevision(ctx context.Context, event OutboxEvent) error
}

// Clock 隔离 dispatcher 的时间读取与轮询 ticker。
type Clock interface {
	Now() time.Time
	NewTicker(interval time.Duration) Ticker
}

// Ticker 是 dispatcher 后台轮询所需的最小 ticker。
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// DispatcherSettings 定义 dispatcher 的运行与退避参数。
type DispatcherSettings struct {
	PollInterval   time.Duration
	BatchSize      int
	ClaimTimeout   time.Duration
	BackoffInitial time.Duration
	BackoffMax     time.Duration
}

// Validate 拒绝不能形成可靠投递循环的配置。
func (s DispatcherSettings) Validate() error {
	switch {
	case s.PollInterval <= 0:
		return errors.New("dispatcher poll interval must be positive")
	case s.BatchSize <= 0:
		return errors.New("dispatcher batch size must be positive")
	case s.ClaimTimeout <= 0:
		return errors.New("dispatcher claim timeout must be positive")
	case s.BackoffInitial <= 0:
		return errors.New("dispatcher initial backoff must be positive")
	case s.BackoffMax <= 0:
		return errors.New("dispatcher maximum backoff must be positive")
	case s.BackoffMax < s.BackoffInitial:
		return errors.New("dispatcher maximum backoff must not be less than initial backoff")
	default:
		return nil
	}
}

// RetryBackoff 返回第 attempt 次失败的有界指数退避，attempt 从 1 开始。
func (s DispatcherSettings) RetryBackoff(attempt int) time.Duration {
	if attempt <= 1 {
		return s.BackoffInitial
	}
	delay := s.BackoffInitial
	for i := 1; i < attempt && delay < s.BackoffMax; i++ {
		if delay > s.BackoffMax/2 {
			return s.BackoffMax
		}
		delay *= 2
	}
	if delay > s.BackoffMax {
		return s.BackoffMax
	}
	return delay
}

// DispatcherStatus 是 dispatcher 与 outbox backlog 的只读状态。
type DispatcherStatus struct {
	Running                bool
	LastSuccessfulDispatch *time.Time
	LastErrorCategory      string
	DueCount               int
	OldestUnfinishedAge    time.Duration
}

// OutboxDispatcherStatus 暴露 health/readiness 可消费的只读状态。
type OutboxDispatcherStatus interface {
	Status(ctx context.Context) (DispatcherStatus, error)
}

// OutboxDispatcherRunner 暴露 lifecycle 所需的显式启停能力。
type OutboxDispatcherRunner interface {
	Start() error
	Stop(ctx context.Context) error
}

func validateDispatcherDependencies(store OutboxStore, publisher PolicyRevisionPublisher, clock Clock) error {
	switch {
	case store == nil:
		return errors.New("dispatcher outbox store is required")
	case publisher == nil:
		return errors.New("dispatcher policy revision publisher is required")
	case clock == nil:
		return errors.New("dispatcher clock is required")
	default:
		return nil
	}
}

func claimLostError(eventID uuid.UUID) error {
	return fmt.Errorf("%w for event %s", ErrOutboxClaimLost, eventID.String())
}
