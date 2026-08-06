package redispubsub

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const testChannel = "runtime:test:notifications"

type inertClient struct {
	subscribeCalls atomic.Int64
	closeCalls     atomic.Int64
}

func (c *inertClient) Subscribe(context.Context, ...string) *redis.PubSub {
	c.subscribeCalls.Add(1)
	return nil
}

func (c *inertClient) Close() error {
	c.closeCalls.Add(1)
	return nil
}

type receiveResult struct {
	value any
	err   error
}

type scriptedSubscription struct {
	receives   chan receiveResult
	closeCalls atomic.Int64
}

func newScriptedSubscription(results ...receiveResult) *scriptedSubscription {
	receives := make(chan receiveResult, len(results))
	for _, result := range results {
		receives <- result
	}
	return &scriptedSubscription{receives: receives}
}

func (s *scriptedSubscription) Receive(ctx context.Context) (any, error) {
	select {
	case result := <-s.receives:
		return result.value, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *scriptedSubscription) Close() error {
	s.closeCalls.Add(1)
	return nil
}

type scriptedFactory struct {
	mu            sync.Mutex
	subscriptions []subscription
	calls         int
}

func (f *scriptedFactory) subscribe(context.Context) subscription {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if len(f.subscriptions) == 0 {
		return newScriptedSubscription()
	}
	subscription := f.subscriptions[0]
	f.subscriptions = f.subscriptions[1:]
	return subscription
}

func (f *scriptedFactory) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func validOptions() Options {
	return Options{
		Name:             "test-subscriber",
		Channel:          testChannel,
		BufferSize:       2,
		SubscribeTimeout: time.Second,
		BackoffInitial:   50 * time.Millisecond,
		BackoffMax:       200 * time.Millisecond,
	}
}

func confirmation() receiveResult {
	return receiveResult{value: &redis.Subscription{Kind: "subscribe", Channel: testChannel, Count: 1}}
}

func TestNewSubscriberValidatesDependenciesAndOptions(t *testing.T) {
	validClient := &inertClient{}
	validLog := zap.NewNop()
	base := validOptions()
	var nilClient *inertClient

	tests := []struct {
		name   string
		client Client
		log    *zap.Logger
		mutate func(*Options)
	}{
		{name: "nil client", log: validLog},
		{name: "typed nil client", client: nilClient, log: validLog},
		{name: "nil logger", client: validClient},
		{name: "empty name", client: validClient, log: validLog, mutate: func(opts *Options) { opts.Name = "" }},
		{name: "blank name", client: validClient, log: validLog, mutate: func(opts *Options) { opts.Name = " \t" }},
		{name: "empty channel", client: validClient, log: validLog, mutate: func(opts *Options) { opts.Channel = "" }},
		{name: "blank channel", client: validClient, log: validLog, mutate: func(opts *Options) { opts.Channel = " \t" }},
		{name: "zero buffer", client: validClient, log: validLog, mutate: func(opts *Options) { opts.BufferSize = 0 }},
		{name: "negative buffer", client: validClient, log: validLog, mutate: func(opts *Options) { opts.BufferSize = -1 }},
		{name: "zero subscribe timeout", client: validClient, log: validLog, mutate: func(opts *Options) { opts.SubscribeTimeout = 0 }},
		{name: "zero initial backoff", client: validClient, log: validLog, mutate: func(opts *Options) { opts.BackoffInitial = 0 }},
		{name: "zero maximum backoff", client: validClient, log: validLog, mutate: func(opts *Options) { opts.BackoffMax = 0 }},
		{name: "inverted backoff", client: validClient, log: validLog, mutate: func(opts *Options) { opts.BackoffMax = opts.BackoffInitial / 2 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := base
			if tt.mutate != nil {
				tt.mutate(&opts)
			}
			subscriber, err := NewSubscriber(tt.client, tt.log, opts)
			require.Error(t, err)
			require.Nil(t, subscriber)
		})
	}
}

func TestNewSubscriberHasNoRuntimeSideEffects(t *testing.T) {
	client := &inertClient{}
	subscriber, err := NewSubscriber(client, zap.NewNop(), validOptions())

	require.NoError(t, err)
	require.Equal(t, int64(0), client.subscribeCalls.Load())
	require.Equal(t, int64(0), client.closeCalls.Load())
	require.Equal(t, Status{State: StateCreated, ErrorCategory: ErrorNone}, subscriber.Status())
	require.Equal(t, validOptions().BufferSize, cap(subscriber.Messages()))
}

func TestSubscriberStartIsIdempotentAndLifecycleIsOneWay(t *testing.T) {
	sub := newScriptedSubscription(confirmation())
	factory := &scriptedFactory{subscriptions: []subscription{sub}}
	subscriber := newSubscriber(factory.subscribe, zap.NewNop(), validOptions())

	require.NoError(t, subscriber.Start())
	require.NoError(t, subscriber.Start())
	require.Eventually(t, func() bool { return subscriber.Status().State == StateConnected }, time.Second, time.Millisecond)
	require.Equal(t, 1, factory.callCount())

	require.NoError(t, subscriber.Stop(context.Background()))
	require.Equal(t, StateStopped, subscriber.Status().State)
	require.ErrorIs(t, subscriber.Start(), ErrStopped)
	_, open := <-subscriber.Messages()
	require.False(t, open)
}

func TestSubscriberReconnectsAndClearsCurrentFailure(t *testing.T) {
	tests := []struct {
		name     string
		first    *scriptedSubscription
		category ErrorCategory
	}{
		{name: "subscribe failure", first: newScriptedSubscription(receiveResult{err: errors.New("confirmation failed")}), category: ErrorSubscribe},
		{name: "confirmation protocol failure", first: newScriptedSubscription(receiveResult{value: "unexpected"}), category: ErrorProtocol},
		{name: "receive failure", first: newScriptedSubscription(confirmation(), receiveResult{err: errors.New("receive failed")}), category: ErrorReceive},
		{name: "receive protocol failure", first: newScriptedSubscription(confirmation(), receiveResult{value: 42}), category: ErrorProtocol},
		{name: "nil subscription", category: ErrorSubscribe},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			second := newScriptedSubscription(confirmation())
			var first subscription
			if tt.first != nil {
				first = tt.first
			}
			factory := &scriptedFactory{subscriptions: []subscription{first, second}}
			subscriber := newSubscriber(factory.subscribe, zap.NewNop(), validOptions())
			require.NoError(t, subscriber.Start())

			require.Eventually(t, func() bool {
				status := subscriber.Status()
				return status.State == StateReconnecting && status.ErrorCategory == tt.category
			}, time.Second, time.Millisecond)
			failed := subscriber.Status()
			require.NotZero(t, failed.LastFailureAt)
			require.Equal(t, uint64(1), failed.Reconnects)

			require.Eventually(t, func() bool { return subscriber.Status().State == StateConnected }, time.Second, time.Millisecond)
			connected := subscriber.Status()
			require.Equal(t, ErrorNone, connected.ErrorCategory)
			require.NotZero(t, connected.LastConnectedAt)
			require.Equal(t, failed.LastFailureAt, connected.LastFailureAt)
			require.Equal(t, 2, factory.callCount())
			require.NoError(t, subscriber.Stop(context.Background()))
			if tt.first != nil {
				require.Equal(t, int64(1), tt.first.closeCalls.Load())
			}
			require.Equal(t, int64(1), second.closeCalls.Load())
		})
	}
}

func TestSubscriberDeliversMessagesInOrder(t *testing.T) {
	sub := newScriptedSubscription(
		confirmation(),
		receiveResult{value: &redis.Message{Channel: testChannel, Payload: "first"}},
		receiveResult{value: &redis.Message{Channel: testChannel, Payload: "second"}},
	)
	factory := &scriptedFactory{subscriptions: []subscription{sub}}
	subscriber := newSubscriber(factory.subscribe, zap.NewNop(), validOptions())
	require.NoError(t, subscriber.Start())

	first := <-subscriber.Messages()
	second := <-subscriber.Messages()
	require.Equal(t, Message{Channel: testChannel, Payload: "first"}, first)
	require.Equal(t, Message{Channel: testChannel, Payload: "second"}, second)
	require.NoError(t, subscriber.Stop(context.Background()))
}

func TestBackoffIsBoundedAndJittered(t *testing.T) {
	maximum := 16 * time.Millisecond
	backoff := time.Millisecond
	require.Equal(t, 2*time.Millisecond, nextBackoff(backoff, maximum))
	backoff = 8 * time.Millisecond
	require.Equal(t, 16*time.Millisecond, nextBackoff(backoff, maximum))
	require.Equal(t, maximum, nextBackoff(maximum, maximum))
	require.Equal(t, maximum, nextBackoff(time.Duration(1<<62), maximum))

	for range 1000 {
		delay := jitteredBackoff(maximum)
		require.GreaterOrEqual(t, delay, maximum/2)
		require.LessOrEqual(t, delay, maximum)
	}
}

func TestSubscriberStopCancelsBlockingPhases(t *testing.T) {
	tests := []struct {
		name      string
		opts      Options
		results   []receiveResult
		waitState State
	}{
		{name: "confirmation", opts: validOptions(), waitState: StateStarting},
		{name: "receive", opts: validOptions(), results: []receiveResult{confirmation()}, waitState: StateConnected},
		{name: "backoff", opts: Options{Name: "backoff", Channel: testChannel, BufferSize: 1, SubscribeTimeout: time.Second, BackoffInitial: time.Hour, BackoffMax: time.Hour}, results: []receiveResult{{err: errors.New("failed")}}, waitState: StateReconnecting},
		{name: "buffer delivery", opts: Options{Name: "buffer", Channel: testChannel, BufferSize: 1, SubscribeTimeout: time.Second, BackoffInitial: time.Second, BackoffMax: time.Second}, results: []receiveResult{confirmation(), {value: &redis.Message{Channel: testChannel, Payload: "one"}}, {value: &redis.Message{Channel: testChannel, Payload: "two"}}}, waitState: StateConnected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := newScriptedSubscription(tt.results...)
			factory := &scriptedFactory{subscriptions: []subscription{sub}}
			subscriber := newSubscriber(factory.subscribe, zap.NewNop(), tt.opts)
			require.NoError(t, subscriber.Start())
			require.Eventually(t, func() bool { return factory.callCount() == 1 }, time.Second, time.Millisecond)
			require.Eventually(t, func() bool { return subscriber.Status().State == tt.waitState }, time.Second, time.Millisecond)
			if tt.name == "buffer delivery" {
				require.Eventually(t, func() bool { return len(subscriber.messages) == 1 }, time.Second, time.Millisecond)
				time.Sleep(10 * time.Millisecond)
			}

			stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			require.NoError(t, subscriber.Stop(stopCtx))
			require.Equal(t, StateStopped, subscriber.Status().State)
			require.Equal(t, int64(1), sub.closeCalls.Load())
			require.Equal(t, 1, factory.callCount())
		})
	}
}

type stubbornSubscription struct {
	receiveCalls   atomic.Int64
	receiveStarted chan struct{}
	receiveRelease chan struct{}
	closeStarted   chan struct{}
	closeRelease   chan struct{}
	closeCalls     atomic.Int64
	startOnce      sync.Once
	closeStartOnce sync.Once
}

func (s *stubbornSubscription) Receive(context.Context) (any, error) {
	if s.receiveCalls.Add(1) == 1 {
		return confirmation().value, nil
	}
	s.startOnce.Do(func() { close(s.receiveStarted) })
	<-s.receiveRelease
	return nil, errors.New("released")
}

func (s *stubbornSubscription) Close() error {
	s.closeCalls.Add(1)
	s.closeStartOnce.Do(func() { close(s.closeStarted) })
	<-s.closeRelease
	return nil
}

func TestSubscriberStopTimeoutKeepsTheSameDrain(t *testing.T) {
	sub := &stubbornSubscription{
		receiveStarted: make(chan struct{}),
		receiveRelease: make(chan struct{}),
		closeStarted:   make(chan struct{}),
		closeRelease:   make(chan struct{}),
	}
	factory := &scriptedFactory{subscriptions: []subscription{sub}}
	subscriber := newSubscriber(factory.subscribe, zap.NewNop(), validOptions())
	require.NoError(t, subscriber.Start())
	<-sub.receiveStarted

	timedOut, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, subscriber.Stop(timedOut), context.DeadlineExceeded)
	<-sub.closeStarted
	require.Equal(t, StateStopping, subscriber.Status().State)

	close(sub.receiveRelease)
	close(sub.closeRelease)
	drainCtx, drainCancel := context.WithTimeout(context.Background(), time.Second)
	defer drainCancel()
	require.NoError(t, subscriber.Stop(drainCtx))
	require.NoError(t, subscriber.Stop(context.Background()))
	require.Equal(t, int64(1), sub.closeCalls.Load())
	require.Equal(t, StateStopped, subscriber.Status().State)
	_, open := <-subscriber.Messages()
	require.False(t, open)
}

func TestSubscriptionAttemptClosesExactlyOnce(t *testing.T) {
	sub := newScriptedSubscription()
	attempt := &subscriptionAttempt{subscription: sub}
	var group sync.WaitGroup
	for range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			attempt.close()
		}()
	}
	group.Wait()
	require.Equal(t, int64(1), sub.closeCalls.Load())
}

func TestStopBeforeStartClosesMessagesAndPreventsStart(t *testing.T) {
	factory := &scriptedFactory{}
	subscriber := newSubscriber(factory.subscribe, zap.NewNop(), validOptions())

	require.NoError(t, subscriber.Stop(context.Background()))
	require.Equal(t, StateStopped, subscriber.Status().State)
	require.ErrorIs(t, subscriber.Start(), ErrStopped)
	require.Equal(t, 0, factory.callCount())
	_, open := <-subscriber.Messages()
	require.False(t, open)
}
