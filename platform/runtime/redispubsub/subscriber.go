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

// close 断开当前消息广播订阅的底层网络连接并释放资源。
func (a *subscriptionAttempt) close() {
	if a == nil || a.subscription == nil {
		return
	}
	a.closeOnce.Do(func() { _ = a.subscription.Close() })
}

// Subscriber 管理单个 Redis 广播通道的订阅建立、消息接收与掉线自动重连。
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

// NewSubscriber 创建消息订阅器对象（仅做参数校验与初始化，不会立即建立网络连接或启动后台协程）。
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

// newSubscriber 创建跨服务消息监听器；只有在显式调用 Start 后才会开始建立网络连接。
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

// validateOptions 检查监听频道名称、缓存连接客户端以及重连时间间隔等参数是否有效。
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

// isNilClient 检查是否传入了有效的缓存客户端，防止空指针导致程序崩溃。
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

// run 在后台持续监听消息广播；一旦网络断开则根据设定的等待时间自动尝试重新连接。
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

// createAttempt 发起一次新的网络订阅连接；如果连接建立失败则记录错误并等待下一次重试。
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

// confirm 确认服务端已经成功受理了本次消息订阅请求；如果确认失败则视为连接异常并准备重试。
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

// receive 持续接收频道里广播推送的新消息并交给业务处理；若网络意外中断则退出并触发重连流程。
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

// clearAttempt 清理当前失效的连接记录，避免后续误接收旧通道的残留废弃消息。
func (s *Subscriber) clearAttempt(attempt *subscriptionAttempt) {
	s.mu.Lock()
	if s.active == attempt {
		s.active = nil
	}
	s.mu.Unlock()
}

// markConnected 将当前监听状态更新为已就绪连接，并清除之前的故障标记。
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

// markFailure 记录监听过程中的故障类型（例如网络连不上、订阅未被受理或传输中断），供系统健康检查判断具体原因。
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

// finishLocked 标记消息监听流程彻底结束，并唤醒所有等待退出的协程。
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

// waitForRetry 在连接断开后稍作等待再进行下一次重连；如果此时收到系统停机通知则立即退出不再等待。
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

// nextBackoff 逐步增加每次重连的等待间隔时间（指数退避），防止因服务故障导致频繁发起重试冲击服务器。
func nextBackoff(current time.Duration, maximum time.Duration) time.Duration {
	if current >= maximum || current > maximum/2 {
		return maximum
	}
	return current * 2
}

// jitteredBackoff 为重连等待时间增加小幅随机抖动，防止大量客户端在完全相同的秒数内同时发起重连冲击系统。
func jitteredBackoff(delay time.Duration) time.Duration {
	half := delay / 2
	if half <= 0 {
		return delay
	}
	return half + time.Duration(rand.Int64N(int64(delay-half)+1)) // #nosec G404 -- 退避抖动不承载安全语义。
}
