package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDrainTrackerWaitsForActiveHandler(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	tracker := newDrainTracker(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	}))
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		tracker.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}()
	requireChannelClosed(t, started, time.Second)

	waitCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, tracker.Wait(waitCtx), context.DeadlineExceeded)

	close(release)
	require.NoError(t, tracker.Wait(context.Background()))
	requireChannelClosed(t, handlerDone, time.Second)
}

func TestDrainTrackerDecrementsAfterHandlerPanic(t *testing.T) {
	t.Parallel()

	tracker := newDrainTracker(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("test panic")
	}))
	func() {
		defer func() { require.Equal(t, "test panic", recover()) }()
		tracker.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}()

	require.NoError(t, tracker.Wait(context.Background()))
	tracker.mu.Lock()
	require.Zero(t, tracker.active)
	tracker.mu.Unlock()
}

func TestDrainTrackerWaitIsWokenByCanceledContext(t *testing.T) {
	t.Parallel()

	tracker := newDrainTracker(http.NotFoundHandler())
	tracker.mu.Lock()
	tracker.active = 1
	tracker.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	waitDone := make(chan error, 1)
	go func() { waitDone <- tracker.Wait(ctx) }()
	cancel()

	select {
	case err := <-waitDone:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("Wait was not woken by context cancellation")
	}
}

func TestDrainTrackerWakesConcurrentWaiters(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	tracker := newDrainTracker(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	}))
	go tracker.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	requireChannelClosed(t, started, time.Second)

	const waiters = 16
	results := make(chan error, waiters)
	var group sync.WaitGroup
	for range waiters {
		group.Add(1)
		go func() {
			defer group.Done()
			results <- tracker.Wait(context.Background())
		}()
	}
	close(release)
	group.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
}
