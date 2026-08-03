package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/aegiscore/common/runtime/datastore"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	"github.com/aegiscore/user-service/internal/persistence/ent/predicate"
	entrbacoutbox "github.com/aegiscore/user-service/internal/persistence/ent/rbacpolicyoutboxevent"
)

type outboxEntTxStarter struct {
	client *ent.Client
}

func (s outboxEntTxStarter) BeginTransaction(ctx context.Context) (*ent.Tx, error) {
	return s.client.Tx(ctx)
}

// OutboxStore 持久化 RBAC policy outbox 的 claim lease 与投递状态。
type OutboxStore struct {
	client *ent.Client
}

var _ permissionapplication.OutboxStore = (*OutboxStore)(nil)
var _ datastore.TransactionStarter[*ent.Tx] = outboxEntTxStarter{}

// NewOutboxStore 构造基于 Ent/PostgreSQL 的 outbox store。
func NewOutboxStore(client *ent.Client) *OutboxStore {
	return &OutboxStore{client: client}
}

// Claim 在短 transaction 内锁定并认领一个按 revision 排序的到期 batch。
func (s *OutboxStore) Claim(ctx context.Context, now time.Time, limit int, claimTimeout time.Duration) (claims []permissionapplication.OutboxClaim, err error) {
	if limit <= 0 {
		return nil, errors.New("outbox claim limit must be positive")
	}
	if claimTimeout <= 0 {
		return nil, errors.New("outbox claim timeout must be positive")
	}

	tx, finish, err := datastore.BeginTransaction(ctx, outboxEntTxStarter{client: s.client})
	if err != nil {
		return nil, fmt.Errorf("begin outbox claim: %w", err)
	}
	defer func() {
		if rollbackErr := finish.RollbackUnlessCommitted(); rollbackErr != nil {
			err = errors.Join(err, rollbackErr)
		}
	}()

	nowMillis := now.UnixMilli()
	rows, err := tx.RbacPolicyOutboxEvent.Query().
		Where(dueOutboxPredicate(nowMillis)).
		Order(entrbacoutbox.ByRevision()).
		Limit(limit).
		ForUpdate(entsql.WithLockAction(entsql.SkipLocked)).
		All(ctx)
	if err != nil {
		return nil, finish.Fail(fmt.Errorf("select due outbox events for claim: %w", err))
	}
	if len(rows) == 0 {
		if err := finish.Commit(ctx); err != nil {
			return nil, finish.Fail(fmt.Errorf("commit empty outbox claim: %w", err))
		}
		return []permissionapplication.OutboxClaim{}, nil
	}

	claimToken, err := uuid.NewRandom()
	if err != nil {
		return nil, finish.Fail(fmt.Errorf("generate outbox claim token: %w", err))
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	updated, err := tx.RbacPolicyOutboxEvent.Update().
		Where(entrbacoutbox.IDIn(ids...)).
		SetStatus(permissionapplication.OutboxStatusProcessing).
		SetClaimToken(claimToken).
		SetClaimedUntil(now.Add(claimTimeout).UnixMilli()).
		SetUpdatedAt(nowMillis).
		Save(ctx)
	if err != nil {
		return nil, finish.Fail(fmt.Errorf("mark outbox events processing: %w", err))
	}
	if updated != len(rows) {
		return nil, finish.Fail(fmt.Errorf("mark outbox events processing: updated %d of %d locked events", updated, len(rows)))
	}
	if err := finish.Commit(ctx); err != nil {
		return nil, finish.Fail(fmt.Errorf("commit outbox claim: %w", err))
	}

	claims = make([]permissionapplication.OutboxClaim, 0, len(rows))
	for _, row := range rows {
		claims = append(claims, permissionapplication.OutboxClaim{
			Event:        toOutboxEvent(row),
			ClaimToken:   claimToken,
			AttemptCount: row.AttemptCount,
		})
	}
	return claims, nil
}

// Ack 仅由仍持有 processing claim token 的 owner 标记 delivered。
func (s *OutboxStore) Ack(ctx context.Context, eventID uuid.UUID, claimToken uuid.UUID, deliveredAt time.Time) (bool, error) {
	updated, err := s.client.RbacPolicyOutboxEvent.Update().
		Where(
			entrbacoutbox.EventIDEQ(eventID),
			entrbacoutbox.StatusEQ(permissionapplication.OutboxStatusProcessing),
			entrbacoutbox.ClaimTokenEQ(claimToken),
		).
		SetStatus(permissionapplication.OutboxStatusDelivered).
		SetDeliveredAt(deliveredAt.UnixMilli()).
		SetUpdatedAt(deliveredAt.UnixMilli()).
		ClearClaimToken().
		ClearClaimedUntil().
		ClearLastError().
		Save(ctx)
	if err != nil {
		return false, fmt.Errorf("ack outbox event %s: %w", eventID.String(), err)
	}
	return updated == 1, nil
}

// Fail 条件记录实际 publish 失败、递增 attempt 并清除当前 claim。
func (s *OutboxStore) Fail(ctx context.Context, eventID uuid.UUID, claimToken uuid.UUID, failedAt time.Time, nextAttemptAt time.Time, errorSummary string) (bool, error) {
	updated, err := s.client.RbacPolicyOutboxEvent.Update().
		Where(
			entrbacoutbox.EventIDEQ(eventID),
			entrbacoutbox.StatusEQ(permissionapplication.OutboxStatusProcessing),
			entrbacoutbox.ClaimTokenEQ(claimToken),
		).
		SetStatus(permissionapplication.OutboxStatusFailed).
		AddAttemptCount(1).
		SetNextAttemptAt(nextAttemptAt.UnixMilli()).
		SetLastError(truncateOutboxError(errorSummary)).
		SetUpdatedAt(failedAt.UnixMilli()).
		ClearClaimToken().
		ClearClaimedUntil().
		Save(ctx)
	if err != nil {
		return false, fmt.Errorf("record outbox event %s failure: %w", eventID.String(), err)
	}
	return updated == 1, nil
}

// Backlog 只读返回到期数量与最老未完成事件时间。
func (s *OutboxStore) Backlog(ctx context.Context, now time.Time) (permissionapplication.OutboxBacklog, error) {
	dueCount, err := s.client.RbacPolicyOutboxEvent.Query().Where(dueOutboxPredicate(now.UnixMilli())).Count(ctx)
	if err != nil {
		return permissionapplication.OutboxBacklog{}, fmt.Errorf("count due outbox events: %w", err)
	}
	oldest, err := s.client.RbacPolicyOutboxEvent.Query().
		Where(entrbacoutbox.StatusNEQ(permissionapplication.OutboxStatusDelivered)).
		Order(entrbacoutbox.ByCreatedAt()).
		First(ctx)
	if ent.IsNotFound(err) {
		return permissionapplication.OutboxBacklog{DueCount: dueCount}, nil
	}
	if err != nil {
		return permissionapplication.OutboxBacklog{}, fmt.Errorf("query oldest unfinished outbox event: %w", err)
	}
	createdAt := time.UnixMilli(oldest.CreatedAt)
	return permissionapplication.OutboxBacklog{DueCount: dueCount, OldestCreatedAt: &createdAt}, nil
}

func dueOutboxPredicate(nowMillis int64) predicate.RbacPolicyOutboxEvent {
	return entrbacoutbox.Or(
		entrbacoutbox.And(
			entrbacoutbox.StatusIn(permissionapplication.OutboxStatusPending, permissionapplication.OutboxStatusFailed),
			entrbacoutbox.NextAttemptAtLTE(nowMillis),
		),
		entrbacoutbox.And(
			entrbacoutbox.StatusEQ(permissionapplication.OutboxStatusProcessing),
			entrbacoutbox.ClaimedUntilLTE(nowMillis),
		),
	)
}

func toOutboxEvent(row *ent.RbacPolicyOutboxEvent) permissionapplication.OutboxEvent {
	return permissionapplication.OutboxEvent{
		EventID:        row.EventID,
		Revision:       row.Revision,
		Kind:           row.Kind,
		Reason:         row.Reason,
		RoleID:         row.RoleID,
		UserID:         row.UserID,
		PermissionID:   row.PermissionID,
		IdempotencyKey: row.IdempotencyKey,
	}
}

func truncateOutboxError(summary string) string {
	const maxLength = 2048
	summary = strings.ToValidUTF8(summary, "?")
	runes := []rune(summary)
	if len(runes) <= maxLength {
		return summary
	}
	return string(runes[:maxLength])
}
