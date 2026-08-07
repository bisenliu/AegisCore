package httpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewValidatesOptions(t *testing.T) {
	t.Parallel()

	base := Options{
		Name:            "api",
		Addr:            "127.0.0.1:0",
		Handler:         http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		ShutdownTimeout: time.Second,
	}
	tests := []struct {
		name   string
		mutate func(*Options)
		field  string
	}{
		{name: "empty name", mutate: func(options *Options) { options.Name = " \t" }, field: "name"},
		{name: "empty addr", mutate: func(options *Options) { options.Addr = " \t" }, field: "addr"},
		{name: "nil handler", mutate: func(options *Options) { options.Handler = nil }, field: "handler"},
		{name: "typed nil handler", mutate: func(options *Options) { options.Handler = http.HandlerFunc(nil) }, field: "handler"},
		{name: "negative read timeout", mutate: func(options *Options) { options.ReadTimeout = -time.Second }, field: "read timeout"},
		{name: "negative write timeout", mutate: func(options *Options) { options.WriteTimeout = -time.Second }, field: "write timeout"},
		{name: "negative idle timeout", mutate: func(options *Options) { options.IdleTimeout = -time.Second }, field: "idle timeout"},
		{name: "zero shutdown timeout", mutate: func(options *Options) { options.ShutdownTimeout = 0 }, field: "shutdown timeout"},
		{name: "negative shutdown timeout", mutate: func(options *Options) { options.ShutdownTimeout = -time.Second }, field: "shutdown timeout"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			options := base
			test.mutate(&options)

			managed, err := New(options)

			require.Nil(t, managed)
			require.ErrorIs(t, err, ErrInvalidOptions)
			require.ErrorContains(t, err, test.field)
		})
	}
}

func TestNewConstructsConfiguredServerWithoutRuntimeResources(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	managed, err := New(Options{
		Name:            " api ",
		Addr:            " 127.0.0.1:0 ",
		Handler:         handler,
		ReadTimeout:     time.Second,
		WriteTimeout:    2 * time.Second,
		IdleTimeout:     3 * time.Second,
		ShutdownTimeout: 4 * time.Second,
	})
	require.NoError(t, err)

	server := managed.HTTPServer()
	require.Equal(t, "127.0.0.1:0", server.Addr)
	require.Equal(t, time.Second, server.ReadTimeout)
	require.Equal(t, 2*time.Second, server.WriteTimeout)
	require.Equal(t, 3*time.Second, server.IdleTimeout)
	require.IsType(t, &drainTracker{}, server.Handler)

	managed.mu.Lock()
	require.Equal(t, stateCreated, managed.state)
	require.Nil(t, managed.listener)
	require.Nil(t, managed.serveDone)
	require.False(t, managed.cleanupStarted)
	managed.mu.Unlock()

	listener, err := net.Listen("tcp", server.Addr)
	require.NoError(t, err, "New must not bind the configured address")
	require.NoError(t, listener.Close())
}

func TestStartFailsSynchronouslyWhenAddressIsOccupied(t *testing.T) {
	t.Parallel()

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, occupied.Close()) })

	var callbackCalls atomic.Int32
	managed := newTestManaged(t, occupied.Addr().String(), http.NotFoundHandler(), time.Second, func(error) {
		callbackCalls.Add(1)
	})

	err = managed.Start(context.Background())

	require.Error(t, err)
	require.ErrorContains(t, err, "listen http server")
	require.ErrorContains(t, err, occupied.Addr().String())
	require.Equal(t, int32(0), callbackCalls.Load())
	managed.mu.Lock()
	require.Equal(t, stateCreated, managed.state)
	require.Nil(t, managed.listener)
	require.Nil(t, managed.serveDone)
	managed.mu.Unlock()
}

func TestStartReturnsAfterAddressIsBound(t *testing.T) {
	t.Parallel()

	managed := newTestManaged(t, "127.0.0.1:0", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}), time.Second, nil)
	require.NoError(t, managed.Start(context.Background()))
	t.Cleanup(func() { _ = managed.Stop(context.Background()) })

	addr := managedListenerAddr(t, managed)
	connection, err := net.DialTimeout("tcp", addr, time.Second)
	require.NoError(t, err)
	require.NoError(t, connection.Close())
}

func TestStartReturnsStableLifecycleErrors(t *testing.T) {
	t.Parallel()

	running := newTestManaged(t, "127.0.0.1:0", http.NotFoundHandler(), time.Second, nil)
	require.NoError(t, running.Start(context.Background()))
	require.ErrorIs(t, running.Start(context.Background()), ErrAlreadyStarted)
	require.NoError(t, running.Stop(context.Background()))
	require.ErrorIs(t, running.Start(context.Background()), ErrStopped)

	neverStarted := newTestManaged(t, "127.0.0.1:0", http.NotFoundHandler(), time.Second, nil)
	require.NoError(t, neverStarted.Stop(context.Background()))
	require.NoError(t, neverStarted.Stop(context.Background()))
	require.ErrorIs(t, neverStarted.Start(context.Background()), ErrStopped)
}

func TestUnexpectedServeErrorCallsCallbackOnceWithoutHoldingLock(t *testing.T) {
	t.Parallel()

	serveErr := errors.New("accept failed")
	callbackDone := make(chan error, 1)
	var callbackCalls atomic.Int32
	var managed *Managed
	managed = newTestManaged(t, "fake:1", http.NotFoundHandler(), time.Second, func(err error) {
		callbackCalls.Add(1)
		require.ErrorIs(t, err, serveErr)
		callbackDone <- managed.Stop(context.Background())
	})
	listener := newErrorListener(serveErr)
	managed.listen = func(context.Context, string, string) (net.Listener, error) {
		return listener, nil
	}

	require.NoError(t, managed.Start(context.Background()))
	select {
	case stopErr := <-callbackDone:
		require.ErrorIs(t, stopErr, serveErr)
	case <-time.After(time.Second):
		t.Fatal("OnServeError blocked while stopping the managed server")
	}
	require.Equal(t, int32(1), callbackCalls.Load())
	require.Equal(t, int32(2), listener.closeCalls.Load())

	secondErr := managed.Stop(context.Background())
	require.ErrorIs(t, secondErr, serveErr)
	require.Equal(t, callbackCalls.Load(), int32(1))
}

func TestServeUnexpectedErrorTransitionsToFailed(t *testing.T) {
	t.Parallel()

	serveErr := errors.New("accept failed")
	callbackDone := make(chan error, 1)
	managed := newTestManaged(t, "fake:direct", http.NotFoundHandler(), time.Second, func(err error) {
		callbackDone <- err
	})
	managed.state = stateRunning
	managed.serveDone = make(chan struct{})

	managed.serve(newErrorListener(serveErr))

	select {
	case callbackErr := <-callbackDone:
		require.ErrorIs(t, callbackErr, serveErr)
		require.ErrorContains(t, callbackErr, "serve http server")
	case <-time.After(time.Second):
		t.Fatal("OnServeError was not called")
	}
	managed.mu.Lock()
	require.Equal(t, stateFailed, managed.state)
	require.ErrorIs(t, managed.serveErr, serveErr)
	serveDone := managed.serveDone
	managed.mu.Unlock()
	requireChannelClosed(t, serveDone, time.Second)
}

func TestNormalStopDoesNotCallServeErrorCallback(t *testing.T) {
	t.Parallel()

	var callbackCalls atomic.Int32
	managed := newTestManaged(t, "127.0.0.1:0", http.NotFoundHandler(), time.Second, func(error) {
		callbackCalls.Add(1)
	})
	require.NoError(t, managed.Start(context.Background()))
	require.NoError(t, managed.Stop(context.Background()))
	require.Equal(t, int32(0), callbackCalls.Load())
}

func TestContextCanceledServeExitDuringStopIsExpected(t *testing.T) {
	t.Parallel()

	listener := newBlockingListener(context.Canceled)
	var callbackCalls atomic.Int32
	managed := newTestManaged(t, "fake:2", http.NotFoundHandler(), time.Second, func(error) {
		callbackCalls.Add(1)
	})
	managed.listen = func(context.Context, string, string) (net.Listener, error) {
		return listener, nil
	}

	require.NoError(t, managed.Start(context.Background()))
	require.NoError(t, managed.Stop(context.Background()))
	require.Equal(t, int32(0), callbackCalls.Load())
}

func TestConcurrentStopSharesOneCleanupAndFinalResult(t *testing.T) {
	t.Parallel()

	realListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	listener := &countingListener{Listener: realListener}
	managed := newTestManaged(t, realListener.Addr().String(), http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}), time.Second, nil)
	managed.listen = func(context.Context, string, string) (net.Listener, error) {
		return listener, nil
	}
	require.NoError(t, managed.Start(context.Background()))

	response, err := http.Get("http://" + realListener.Addr().String())
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())

	const callers = 24
	start := make(chan struct{})
	results := make(chan error, callers)
	var waiters sync.WaitGroup
	for range callers {
		waiters.Add(1)
		go func() {
			defer waiters.Done()
			<-start
			results <- managed.Stop(context.Background())
		}()
	}
	close(start)
	waiters.Wait()
	close(results)
	for stopErr := range results {
		require.NoError(t, stopErr)
	}
	require.Equal(t, int32(2), listener.closeCalls.Load(), "Shutdown and explicit listener cleanup must run once each")
}

func TestCallerStopTimeoutDoesNotCancelSharedCleanup(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	managed := newTestManaged(t, "127.0.0.1:0", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(writer, "ok")
	}), time.Second, nil)
	require.NoError(t, managed.Start(context.Background()))
	requestDone := startHTTPTestRequest(managedListenerAddr(t, managed))
	requireChannelClosed(t, started, time.Second)

	firstCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, managed.Stop(firstCtx), context.DeadlineExceeded)

	secondDone := make(chan error, 1)
	go func() { secondDone <- managed.Stop(context.Background()) }()
	require.Never(t, func() bool {
		select {
		case <-secondDone:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, 5*time.Millisecond)
	close(release)
	require.NoError(t, <-secondDone)
	require.NoError(t, <-requestDone)
	require.NoError(t, managed.Stop(context.Background()))
}

func TestStopGracefullyDrainsSlowHandler(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	managed := newTestManaged(t, "127.0.0.1:0", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		writer.WriteHeader(http.StatusNoContent)
	}), time.Second, nil)
	require.NoError(t, managed.Start(context.Background()))
	requestDone := startHTTPTestRequest(managedListenerAddr(t, managed))
	requireChannelClosed(t, started, time.Second)

	stopDone := make(chan error, 1)
	go func() { stopDone <- managed.Stop(context.Background()) }()
	require.Never(t, func() bool {
		select {
		case <-stopDone:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, 5*time.Millisecond)
	close(release)
	require.NoError(t, <-stopDone)
	require.NoError(t, <-requestDone)
}

func TestStopForcesCloseAfterShutdownTimeout(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	exited := make(chan struct{})
	managed := newTestManaged(t, "127.0.0.1:0", http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		close(started)
		<-request.Context().Done()
		close(exited)
	}), 30*time.Millisecond, nil)
	require.NoError(t, managed.Start(context.Background()))
	requestDone := startHTTPTestRequest(managedListenerAddr(t, managed))
	requireChannelClosed(t, started, time.Second)

	err := managed.Stop(context.Background())

	require.ErrorIs(t, err, context.DeadlineExceeded)
	requireChannelClosed(t, exited, time.Second)
	select {
	case <-requestDone:
	case <-time.After(time.Second):
		t.Fatal("forced Close did not release the client request")
	}
}

func TestStopPreservesDrainTimeoutWhenHandlerIgnoresCancellation(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	managed := newTestManaged(t, "127.0.0.1:0", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	}), 30*time.Millisecond, nil)
	require.NoError(t, managed.Start(context.Background()))
	requestDone := startHTTPTestRequest(managedListenerAddr(t, managed))
	requireChannelClosed(t, started, time.Second)

	err := managed.Stop(context.Background())

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.ErrorContains(t, err, "wait handlers")
	require.EqualError(t, managed.Stop(context.Background()), err.Error())
	close(release)
	select {
	case <-requestDone:
	case <-time.After(time.Second):
		t.Fatal("handler did not return after test release")
	}
}

func TestStartImmediatelyFollowedByStopReleasesResources(t *testing.T) {
	t.Parallel()

	for range 50 {
		listener := newBlockingListener(net.ErrClosed)
		managed := newTestManaged(t, "fake:3", http.NotFoundHandler(), time.Second, nil)
		managed.listen = func(context.Context, string, string) (net.Listener, error) {
			return listener, nil
		}

		require.NoError(t, managed.Start(context.Background()))
		require.NoError(t, managed.Stop(context.Background()))
		require.Equal(t, int32(2), listener.closeCalls.Load())
		managed.mu.Lock()
		require.Equal(t, stateStopped, managed.state)
		serveDone := managed.serveDone
		managed.mu.Unlock()
		requireChannelClosed(t, serveDone, time.Second)
	}
}

func newTestManaged(
	t testing.TB,
	addr string,
	handler http.Handler,
	shutdownTimeout time.Duration,
	onServeError func(error),
) *Managed {
	t.Helper()
	managed, err := New(Options{
		Name:            "test",
		Addr:            addr,
		Handler:         handler,
		ShutdownTimeout: shutdownTimeout,
		OnServeError:    onServeError,
	})
	require.NoError(t, err)
	return managed
}

func managedListenerAddr(t testing.TB, managed *Managed) string {
	t.Helper()
	managed.mu.Lock()
	defer managed.mu.Unlock()
	require.NotNil(t, managed.listener)
	return managed.listener.Addr().String()
}

func startHTTPTestRequest(addr string) <-chan error {
	done := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		response, err := client.Get("http://" + addr)
		if err != nil {
			done <- err
			return
		}
		defer response.Body.Close()
		_, err = io.Copy(io.Discard, response.Body)
		done <- err
	}()
	return done
}

func requireChannelClosed(t testing.TB, channel <-chan struct{}, timeout time.Duration) {
	t.Helper()
	select {
	case <-channel:
	case <-time.After(timeout):
		t.Fatal("channel was not closed before timeout")
	}
}

type errorListener struct {
	err        error
	closeCalls atomic.Int32
}

func newErrorListener(err error) *errorListener {
	return &errorListener{err: err}
}

func (l *errorListener) Accept() (net.Conn, error) { return nil, l.err }
func (l *errorListener) Addr() net.Addr            { return testAddr("error-listener") }
func (l *errorListener) Close() error {
	l.closeCalls.Add(1)
	return nil
}

type blockingListener struct {
	err        error
	closed     chan struct{}
	closeOnce  sync.Once
	closeCalls atomic.Int32
}

func newBlockingListener(err error) *blockingListener {
	return &blockingListener{err: err, closed: make(chan struct{})}
}

func (l *blockingListener) Accept() (net.Conn, error) {
	<-l.closed
	return nil, l.err
}

func (l *blockingListener) Addr() net.Addr { return testAddr("blocking-listener") }

func (l *blockingListener) Close() error {
	l.closeCalls.Add(1)
	l.closeOnce.Do(func() { close(l.closed) })
	return nil
}

type countingListener struct {
	net.Listener
	closeCalls atomic.Int32
}

func (l *countingListener) Close() error {
	l.closeCalls.Add(1)
	return l.Listener.Close()
}

type testAddr string

func (a testAddr) Network() string { return "tcp" }
func (a testAddr) String() string  { return string(a) }

func TestErrorsContainServerIdentityAndOperation(t *testing.T) {
	t.Parallel()

	managed := newTestManaged(t, "bad address", http.NotFoundHandler(), time.Second, nil)
	err := managed.Start(context.Background())
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "test") && strings.Contains(err.Error(), "bad address"))
	require.Contains(t, err.Error(), "listen")
}

func TestDrainTrackerIsUsedByHTTPServer(t *testing.T) {
	t.Parallel()

	called := false
	managed := newTestManaged(t, "127.0.0.1:0", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}), time.Second, nil)
	managed.HTTPServer().Handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	require.True(t, called)
	require.Equal(t, 0, managed.drain.active)
}

func TestLifecycleErrorIncludesSentinel(t *testing.T) {
	t.Parallel()

	managed := newTestManaged(t, "127.0.0.1:0", http.NotFoundHandler(), time.Second, nil)
	require.NoError(t, managed.Start(context.Background()))
	err := managed.Start(context.Background())
	require.ErrorIs(t, err, ErrAlreadyStarted)
	require.Contains(t, fmt.Sprint(err), "start http server")
	require.NoError(t, managed.Stop(context.Background()))
}
