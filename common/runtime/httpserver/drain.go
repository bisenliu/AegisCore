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

func newDrainTracker(handler http.Handler) *drainTracker {
	tracker := &drainTracker{handler: handler}
	tracker.cond = sync.NewCond(&tracker.mu)
	return tracker
}

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
