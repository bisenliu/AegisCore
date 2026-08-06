// Package redispubsub 提供业务中立的 Redis classic Pub/Sub 单 channel 订阅生命周期。
//
// Subscriber 显式等待订阅确认，在接收失败后执行有界指数退避重连，并通过固定容量
// channel 向调用方施加背压。Redis Pub/Sub 仍是可丢失的 at-most-once 通知机制；本包
// 不提供业务消息格式、发布封装、持久化、幂等或可靠投递能力。
package redispubsub
