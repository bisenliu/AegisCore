package nacos

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// LoadConfigDocument 按声明顺序尝试 Nacos server，直到某个 server 返回指定 dataId 内容。
// 整体请求受 env.Timeout 约束，单个 server 失败会保留 server origin 后聚合返回。
func (l *v3Loader) LoadConfigDocument(ctx context.Context, env Env, dataID string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, env.Timeout)
	defer cancel()

	var attempts []error
	for index, server := range l.servers {
		if err := ctx.Err(); err != nil {
			attempts = append(attempts, err)
			break
		}
		// 给剩余 server 均分当前剩余预算，避免首个故障节点阻塞完整超时时间。
		attemptTimeout := attemptTimeout(ctx, len(l.servers)-index)
		attemptCtx, cancelAttempt := context.WithTimeout(ctx, attemptTimeout)
		content, err := l.loadFromServer(attemptCtx, server, env, dataID)
		cancelAttempt()
		if err == nil {
			return content, nil
		}
		attempts = append(attempts, fmt.Errorf("%s: %w", serverOrigin(server), err))
	}
	return nil, fmt.Errorf("all nacos servers failed: %w", errors.Join(attempts...))
}

// attemptTimeout 将剩余总预算平均分配给尚未尝试的服务器，避免单个故障节点耗尽全部预算。
func attemptTimeout(ctx context.Context, serversRemaining int) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok || serversRemaining <= 1 {
		return time.Until(deadline)
	}
	remaining := time.Until(deadline)
	budget := remaining / time.Duration(serversRemaining)
	if budget <= 0 {
		return remaining
	}
	return budget
}
