package redispubsub

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrStopped 表示 subscriber 已开始停止，生命周期不能重新启动。
var ErrStopped = errors.New("redis pubsub subscriber is stopped")

// Client 是创建 classic Pub/Sub 订阅所需的最小 Redis client 接口。
type Client interface {
	Subscribe(ctx context.Context, channels ...string) *redis.PubSub
}

// Options 描述单 channel subscriber 的全部必需配置。
type Options struct {
	Name             string
	Channel          string
	BufferSize       int
	SubscribeTimeout time.Duration
	BackoffInitial   time.Duration
	BackoffMax       time.Duration
}

// Message 是 subscriber 向调用方交付的 Redis Pub/Sub 消息。
type Message struct {
	Channel string
	Pattern string
	Payload string
}

// State 表示 subscriber 的单向生命周期状态。
type State string

const (
	// StateCreated 表示 subscriber 已构造但尚未启动。
	StateCreated State = "created"
	// StateStarting 表示 subscriber 正在等待首次订阅确认。
	StateStarting State = "starting"
	// StateConnected 表示 subscriber 已确认当前订阅。
	StateConnected State = "connected"
	// StateReconnecting 表示 subscriber 正在失败后退避或重建订阅。
	StateReconnecting State = "reconnecting"
	// StateStopping 表示 subscriber 已取消 root context 且正在等待资源退出。
	StateStopping State = "stopping"
	// StateStopped 表示 subscriber 已完全停止。
	StateStopped State = "stopped"
)

// ErrorCategory 表示 subscriber 当前故障的低基数类别。
type ErrorCategory string

const (
	// ErrorNone 表示当前没有订阅故障。
	ErrorNone ErrorCategory = "none"
	// ErrorSubscribe 表示创建订阅或等待确认失败。
	ErrorSubscribe ErrorCategory = "subscribe_failed"
	// ErrorReceive 表示已确认订阅的 Receive 失败。
	ErrorReceive ErrorCategory = "receive_failed"
	// ErrorProtocol 表示 Redis 返回了不符合单 channel classic Pub/Sub 契约的消息。
	ErrorProtocol ErrorCategory = "protocol_failed"
)

// Status 是 subscriber 当前结构化状态的只读快照。
type Status struct {
	Running         bool
	State           State
	ErrorCategory   ErrorCategory
	LastConnectedAt time.Time
	LastFailureAt   time.Time
	Reconnects      uint64
}
