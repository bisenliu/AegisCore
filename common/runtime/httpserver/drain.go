package httpserver

import (
	"context"
	"net/http"
	"sync"
)

type drainTracker struct {
	handler http.Handler

	mu     sync.Mutex
	cond   *sync.Cond
	active int
}

// newDrainTracker 创建正在处理的请求追踪器，用于在系统准备停止时准确统计还有多少用户请求尚未处理完毕。
func newDrainTracker(handler http.Handler) *drainTracker {
	tracker := &drainTracker{handler: handler}
	tracker.cond = sync.NewCond(&tracker.mu)
	return tracker
}

// ServeHTTP 拦截处理到来的网络请求，在请求开始前将正在处理的计数加一，请求结束后减一，并提醒等待停止的流程。
func (t *drainTracker) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	t.mu.Lock()
	t.active++
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		t.active--
		if t.active == 0 {
			t.cond.Broadcast()
		}
		t.mu.Unlock()
	}()

	t.handler.ServeHTTP(writer, request)
}

// Wait 等待当前所有正在处理中的请求全部完成；如果超过等待时限仍未结束，则返回超时未完成错误。
func (t *drainTracker) Wait(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	stopContextWakeup := context.AfterFunc(ctx, func() {
		t.mu.Lock()
		t.cond.Broadcast()
		t.mu.Unlock()
	})
	defer stopContextWakeup()

	t.mu.Lock()
	defer t.mu.Unlock()
	for t.active > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		t.cond.Wait()
	}
	return nil
}
