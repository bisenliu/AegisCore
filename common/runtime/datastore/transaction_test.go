package datastore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type transactionContextKey struct{}

func TestBeginTransactionUsesDetachedLifecycleContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), transactionContextKey{}, "request-value"))
	cancel()
	starter := &testTransactionStarter{tx: &testTransaction{}}

	_, finish, err := BeginTransaction(ctx, starter)

	require.NoError(t, err)
	require.NotNil(t, finish)
	require.Equal(t, "request-value", starter.ctx.Value(transactionContextKey{}))
	require.NoError(t, starter.ctx.Err())
	_, ok := starter.ctx.Deadline()
	require.False(t, ok)
}

func TestBeginTransactionInheritsOriginalDeadline(t *testing.T) {
	wantDeadline := time.Now().Add(30 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), wantDeadline)
	t.Cleanup(cancel)
	starter := &testTransactionStarter{tx: &testTransaction{}}

	_, _, err := BeginTransaction(ctx, starter)

	require.NoError(t, err)
	deadline, ok := starter.ctx.Deadline()
	require.True(t, ok)
	require.Equal(t, wantDeadline, deadline)
}

func TestBeginTransactionFailureCancelsLifecycleContext(t *testing.T) {
	beginErr := errors.New("begin failed")
	starter := &testTransactionStarter{err: beginErr}

	_, finish, err := BeginTransaction(context.Background(), starter)

	require.ErrorIs(t, err, beginErr)
	require.Nil(t, finish)
	require.ErrorIs(t, starter.ctx.Err(), context.Canceled)
}

func TestCommitCancelsLifecycleContext(t *testing.T) {
	starter := &testTransactionStarter{tx: &testTransaction{}}

	_, finish, err := BeginTransaction(context.Background(), starter)
	require.NoError(t, err)

	require.NoError(t, finish.Commit(context.Background()))
	require.ErrorIs(t, starter.ctx.Err(), context.Canceled)
}

func TestFailCancelsLifecycleContext(t *testing.T) {
	starter := &testTransactionStarter{tx: &testTransaction{}}
	businessErr := errors.New("business failed")

	_, finish, err := BeginTransaction(context.Background(), starter)
	require.NoError(t, err)

	require.ErrorIs(t, finish.Fail(businessErr), businessErr)
	require.ErrorIs(t, starter.ctx.Err(), context.Canceled)
}

func TestRollbackUnlessCommittedCancelsLifecycleContext(t *testing.T) {
	starter := &testTransactionStarter{tx: &testTransaction{}}

	_, finish, err := BeginTransaction(context.Background(), starter)
	require.NoError(t, err)

	require.NoError(t, finish.RollbackUnlessCommitted())
	require.ErrorIs(t, starter.ctx.Err(), context.Canceled)
}

func TestRollbackUnlessCommittedRunsAfterRequestCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tx := &testTransaction{}
	_, finish, err := BeginTransaction(ctx, &testTransactionStarter{tx: tx})
	require.NoError(t, err)

	cancel()

	require.NoError(t, finish.RollbackUnlessCommitted())
	require.Equal(t, 1, tx.rollbackCount)
	require.Zero(t, tx.commitCount)
}

func TestCommitRejectsCanceledRequestContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tx := &testTransaction{}
	starter := &testTransactionStarter{tx: tx}
	_, finish, err := BeginTransaction(ctx, starter)
	require.NoError(t, err)

	cancel()

	require.ErrorIs(t, finish.Commit(ctx), context.Canceled)
	require.Zero(t, tx.commitCount)
	require.NoError(t, starter.ctx.Err())
	require.NoError(t, finish.RollbackUnlessCommitted())
	require.Equal(t, 1, tx.rollbackCount)
	require.ErrorIs(t, starter.ctx.Err(), context.Canceled)
}

func TestCommitWithoutOriginalDeadlineSucceedsAfterCleanupTimeout(t *testing.T) {
	tx := &testTransaction{}
	_, finish, err := BeginTransaction(context.Background(), &testTransactionStarter{tx: tx})
	require.NoError(t, err)

	time.Sleep(DefaultTransactionCleanupTimeout + 20*time.Millisecond)

	require.NoError(t, finish.Commit(context.Background()))
	require.Equal(t, 1, tx.commitCount)
	require.Zero(t, tx.rollbackCount)
}

func TestFailJoinsOriginalAndRollbackErrors(t *testing.T) {
	businessErr := errors.New("business failed")
	rollbackErr := errors.New("rollback failed")
	tx := &testTransaction{rollbackErr: rollbackErr}
	_, finish, err := BeginTransaction(context.Background(), &testTransactionStarter{tx: tx})
	require.NoError(t, err)

	err = finish.Fail(businessErr)

	require.ErrorIs(t, err, businessErr)
	require.ErrorIs(t, err, rollbackErr)
	require.Equal(t, 1, tx.rollbackCount)
	require.NoError(t, finish.RollbackUnlessCommitted())
	require.Equal(t, 1, tx.rollbackCount)
}

func TestCommitFailureAllowsDeferredRollback(t *testing.T) {
	commitErr := errors.New("commit failed")
	tx := &testTransaction{commitErr: commitErr}
	_, finish, err := BeginTransaction(context.Background(), &testTransactionStarter{tx: tx})
	require.NoError(t, err)

	err = finish.Commit(context.Background())

	require.ErrorIs(t, err, commitErr)
	require.Equal(t, 1, tx.commitCount)
	require.Zero(t, tx.rollbackCount)
	require.NoError(t, finish.RollbackUnlessCommitted())
	require.Equal(t, 1, tx.rollbackCount)
}

type testTransactionStarter struct {
	tx  *testTransaction
	ctx context.Context
	err error
}

func (s *testTransactionStarter) BeginTransaction(ctx context.Context) (*testTransaction, error) {
	s.ctx = ctx
	if s.err != nil {
		return nil, s.err
	}
	return s.tx, nil
}

type testTransaction struct {
	commitCount   int
	rollbackCount int
	commitErr     error
	rollbackErr   error
}

func (tx *testTransaction) Commit() error {
	tx.commitCount++
	return tx.commitErr
}

func (tx *testTransaction) Rollback() error {
	tx.rollbackCount++
	return tx.rollbackErr
}
