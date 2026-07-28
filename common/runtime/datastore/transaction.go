package datastore

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const DefaultTransactionCleanupTimeout = 5 * time.Second

// Transaction 表达业务中立的最小事务终结接口。
type Transaction interface {
	Commit() error
	Rollback() error
}

// TransactionStarter 使用调用方提供的 lifecycle context 开始事务。
type TransactionStarter[T Transaction] interface {
	BeginTransaction(context.Context) (T, error)
}

// Finish 负责统一提交、失败回滚和 defer 兜底回滚语义。
type Finish[T Transaction] struct {
	tx        T
	cancel    context.CancelFunc
	committed bool
	done      bool
}

// BeginTransaction 使用 detached lifecycle context 创建事务。
func BeginTransaction[T Transaction](ctx context.Context, starter TransactionStarter[T]) (T, *Finish[T], error) {
	txCtx, cancel := transactionLifecycleContext(ctx)

	tx, err := starter.BeginTransaction(txCtx)
	if err != nil {
		cancel()
		var zero T
		return zero, nil, fmt.Errorf("begin transaction: %w", err)
	}

	return tx, &Finish[T]{
		tx:     tx,
		cancel: cancel,
	}, nil
}

// Commit 在原始 request context 未取消时提交事务。
func (f *Finish[T]) Commit(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := f.tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	f.committed = true
	f.done = true
	f.cancel()
	return nil
}

// Fail 回滚事务，并在 rollback 失败时保留原始错误和回滚错误。
func (f *Finish[T]) Fail(err error) error {
	if rollbackErr := f.rollback(); rollbackErr != nil {
		return errors.Join(err, rollbackErr)
	}
	return err
}

// RollbackUnlessCommitted 在事务尚未提交或终结时回滚。
func (f *Finish[T]) RollbackUnlessCommitted() error {
	if f.committed || f.done {
		return nil
	}
	return f.rollback()
}

func (f *Finish[T]) rollback() error {
	defer f.cancel()
	if f.done {
		return nil
	}

	f.done = true
	if err := f.tx.Rollback(); err != nil {
		return fmt.Errorf("rollback transaction: %w", err)
	}
	return nil
}

func transactionLifecycleContext(ctx context.Context) (context.Context, context.CancelFunc) {
	txCtx := context.WithoutCancel(ctx)

	if deadline, ok := ctx.Deadline(); ok {
		return context.WithDeadline(txCtx, deadline)
	}

	return context.WithTimeout(txCtx, DefaultTransactionCleanupTimeout)
}
