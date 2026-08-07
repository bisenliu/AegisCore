package redispubsub

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type subscription interface {
	Receive(context.Context) (any, error)
	Close() error
}

type subscriptionFactory func(context.Context) subscription

type subscriptionAttempt struct {
	subscription subscription
	closeOnce    sync.Once
}

func (a *subscriptionAttempt) close() {
	if a == nil || a.subscription == nil {
		return
	}
	a.closeOnce.Do(func() { _ = a.subscription.Close() })
}

// Subscriber 管理一个 Redis classic Pub/Sub channel 的订阅与重连生命周期。
type Subscriber struct {
	subscribe subscriptionFactory
	log       *zap.Logger
	opts      Options
	messages  chan Message

	mu        sync.Mutex
	status    Status
	cancel    context.CancelFunc
	done      chan struct{}
	active    *subscriptionAttempt
	closeOnce sync.Once
}

// NewSubscriber 构造 subscriber，但不连接 Redis、创建订阅或启动 goroutine。
func NewSubscriber(client Client, log *zap.Logger, opts Options) (*Subscriber, error) {
	if isNilClient(client) {
		return nil, errors.New("redis pubsub client is required")
	}
	if log == nil {
		return nil, errors.New("redis pubsub logger is required")
	}
	if err := validateOptions(opts); err != nil {
		return nil, err
	}
	return newSubscriber(func(ctx context.Context) subscription {
		pubsub := client.Subscribe(ctx, opts.Channel)
		if pubsub == nil {
			return nil
		}
		return pubsub
	}, log, opts), nil
}

func newSubscriber(subscribe subscriptionFactory, log *zap.Logger, opts Options) *Subscriber {
	return &Subscriber{
		subscribe: subscribe,
		log:       log,
		opts:      opts,
		messages:  make(chan Message, opts.BufferSize),
		status: Status{
			State:         StateCreated,
			ErrorCategory: ErrorNone,
		},
	}
}

func validateOptions(opts Options) error {
	switch {
	case strings.TrimSpace(opts.Name) == "":
		return errors.New("redis pubsub name is required")
	case strings.TrimSpace(opts.Channel) == "":
		return errors.New("redis pubsub channel is required")
	case opts.BufferSize <= 0:
		return errors.New("redis pubsub buffer size must be positive")
	case opts.SubscribeTimeout <= 0:
		return errors.New("redis pubsub subscribe timeout must be positive")
	case opts.BackoffInitial <= 0:
		return errors.New("redis pubsub initial backoff must be positive")
	case opts.BackoffMax <= 0:
		return errors.New("redis pubsub maximum backoff must be positive")
	case opts.BackoffMax < opts.BackoffInitial:
		return errors.New("redis pubsub maximum backoff must not be less than initial backoff")
	default:
		return nil
	}
}

func isNilClient(client Client) bool {
	if client == nil {
		return true
	}
	value := reflect.ValueOf(client)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// Start 启动订阅；运行期重复调用幂等，停止开始后返回 ErrStopped。
func (s *Subscriber) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("redis pubsub subscriber start context is required")
	}
	s.mu.Lock()
	switch s.status.State {
	case StateCreated:
		runCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		s.cancel = cancel
		s.done = done
		s.status.Running = true
		s.status.State = StateStarting
		s.mu.Unlock()
		go s.run(runCtx, done)
		return nil
	case StateStarting, StateConnected, StateReconnecting:
		s.mu.Unlock()
		return nil
	case StateStopping, StateStopped:
		s.mu.Unlock()
		return ErrStopped
	default:
		s.mu.Unlock()
		return fmt.Errorf("invalid redis pubsub subscriber state %q", s.status.State)
	}
}

// Messages 返回 subscriber 唯一的只读消息 channel。
func (s *Subscriber) Messages() <-chan Message {
	return s.messages
}

// Status 返回 subscriber 当前结构化状态的并发安全快照。
func (s *Subscriber) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Stop 单向停止 subscriber，并在调用方 context 期限内等待同一个后台 drain。
func (s *Subscriber) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	state := s.status.State
	if state == StateCreated {
		done := make(chan struct{})
		s.done = done
		s.status.State = StateStopping
		s.status.Running = true
		s.finishLocked(done)
		s.mu.Unlock()
		return nil
	}
	if state == StateStopped {
		s.mu.Unlock()
		return nil
	}
	first := state != StateStopping
	if first {
		s.status.State = StateStopping
	}
	cancel := s.cancel
	done := s.done
	active := s.active
	s.mu.Unlock()

	if first {
		cancel()
		if active != nil {
			// Close 没有 context；放到 drain 路径执行，避免阻塞 Stop 的调用方期限。
			go active.close()
		}
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Subscriber) run(ctx context.Context, done chan struct{}) {
	defer func() {
		s.mu.Lock()
		active := s.active
		s.mu.Unlock()
		active.close()

		s.mu.Lock()
		s.active = nil
		s.finishLocked(done)
		s.mu.Unlock()
	}()

	backoff := s.opts.BackoffInitial
	for ctx.Err() == nil {
		attempt := s.createAttempt(ctx)
		if attempt == nil {
			return
		}

		category, err := s.confirm(ctx, attempt)
		if err == nil {
			s.markConnected()
			backoff = s.opts.BackoffInitial
			category, err = s.receive(ctx, attempt)
		}

		attempt.close()
		s.clearAttempt(attempt)
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			return
		}
		s.markFailure(category)
		s.log.Error("redis pubsub subscription failed",
			zap.String("subscriber_name", s.opts.Name),
			zap.String("channel", s.opts.Channel),
			zap.String("error_category", string(category)),
			zap.Error(err),
		)
		if !waitForRetry(ctx, jitteredBackoff(backoff)) {
			return
		}
		backoff = nextBackoff(backoff, s.opts.BackoffMax)
	}
}

func (s *Subscriber) createAttempt(ctx context.Context) *subscriptionAttempt {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil || s.status.State == StateStopping || s.status.State == StateStopped {
		return nil
	}
	attempt := &subscriptionAttempt{subscription: s.subscribe(ctx)}
	s.active = attempt
	return attempt
}

func (s *Subscriber) confirm(ctx context.Context, attempt *subscriptionAttempt) (ErrorCategory, error) {
	if attempt.subscription == nil {
		return ErrorSubscribe, errors.New("redis pubsub subscribe returned nil PubSub")
	}
	confirmCtx, cancel := context.WithTimeout(ctx, s.opts.SubscribeTimeout)
	defer cancel()
	received, err := attempt.subscription.Receive(confirmCtx)
	if err != nil {
		return ErrorSubscribe, err
	}
	confirmation, ok := received.(*redis.Subscription)
	if !ok || confirmation.Kind != "subscribe" || confirmation.Channel != s.opts.Channel {
		return ErrorProtocol, fmt.Errorf("unexpected redis pubsub confirmation: %T", received)
	}
	return ErrorNone, nil
}

func (s *Subscriber) receive(ctx context.Context, attempt *subscriptionAttempt) (ErrorCategory, error) {
	for {
		received, err := attempt.subscription.Receive(ctx)
		if err != nil {
			return ErrorReceive, err
		}
		switch message := received.(type) {
		case *redis.Message:
			if message.Channel != s.opts.Channel || message.Pattern != "" {
				return ErrorProtocol, fmt.Errorf("unexpected redis pubsub message channel %q pattern %q", message.Channel, message.Pattern)
			}
			select {
			case s.messages <- Message{Channel: message.Channel, Pattern: message.Pattern, Payload: message.Payload}:
			case <-ctx.Done():
				return ErrorReceive, ctx.Err()
			}
		case *redis.Subscription:
			if message.Kind != "subscribe" || message.Channel != s.opts.Channel {
				return ErrorProtocol, fmt.Errorf("unexpected redis pubsub subscription event: kind=%q channel=%q", message.Kind, message.Channel)
			}
			s.markConnected()
		case *redis.Pong:
		default:
			return ErrorProtocol, fmt.Errorf("unexpected redis pubsub message: %T", received)
		}
	}
}

func (s *Subscriber) clearAttempt(attempt *subscriptionAttempt) {
	s.mu.Lock()
	if s.active == attempt {
		s.active = nil
	}
	s.mu.Unlock()
}

func (s *Subscriber) markConnected() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status.State == StateStopping || s.status.State == StateStopped {
		return
	}
	s.status.State = StateConnected
	s.status.ErrorCategory = ErrorNone
	s.status.LastConnectedAt = time.Now()
}

func (s *Subscriber) markFailure(category ErrorCategory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status.State == StateStopping || s.status.State == StateStopped {
		return
	}
	s.status.State = StateReconnecting
	s.status.ErrorCategory = category
	s.status.LastFailureAt = time.Now()
	s.status.Reconnects++
}

func (s *Subscriber) finishLocked(done chan struct{}) {
	if s.done != done || s.status.State == StateStopped {
		return
	}
	s.cancel = nil
	s.status.Running = false
	s.status.State = StateStopped
	s.status.ErrorCategory = ErrorNone
	s.closeOnce.Do(func() { close(s.messages) })
	close(done)
}

func waitForRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextBackoff(current time.Duration, maximum time.Duration) time.Duration {
	if current >= maximum || current > maximum/2 {
		return maximum
	}
	return current * 2
}

func jitteredBackoff(delay time.Duration) time.Duration {
	half := delay / 2
	if half <= 0 {
		return delay
	}
	return half + time.Duration(rand.Int64N(int64(delay-half)+1)) // #nosec G404 -- 退避抖动不承载安全语义。
}
