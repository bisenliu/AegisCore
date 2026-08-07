package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type serverState uint8

const (
	stateCreated serverState = iota
	stateRunning
	stateFailed
	stateStopping
	stateStopped
)

type listenFunc func(context.Context, string, string) (net.Listener, error)

// Managed 管理一个 net/http server 的监听、服务与关闭生命周期。
//
// Managed 不支持重启。Stop 可由多个 goroutine 并发或重复调用，所有调用
// 共享同一个后台清理流程和最终结果。
type Managed struct {
	name            string
	addr            string
	shutdownTimeout time.Duration
	onServeError    func(error)
	server          *http.Server
	drain           *drainTracker
	listen          listenFunc

	mu             sync.Mutex
	state          serverState
	listener       net.Listener
	serveDone      chan struct{}
	cleanupDone    chan struct{}
	cleanupStarted bool
	serveErr       error
	stopErr        error
}

// New 校验 options 并创建尚未启动的 Managed。
// New 不监听端口，也不创建 goroutine。
func New(options Options) (*Managed, error) {
	if err := validateOptions(options); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(options.Name)
	addr := strings.TrimSpace(options.Addr)
	drain := newDrainTracker(options.Handler)
	managed := &Managed{
		name:            name,
		addr:            addr,
		shutdownTimeout: options.ShutdownTimeout,
		onServeError:    options.OnServeError,
		drain:           drain,
		state:           stateCreated,
		cleanupDone:     make(chan struct{}),
	}
	managed.server = &http.Server{
		Addr:         addr,
		Handler:      drain,
		ReadTimeout:  options.ReadTimeout,
		WriteTimeout: options.WriteTimeout,
		IdleTimeout:  options.IdleTimeout,
	}
	managed.listen = func(ctx context.Context, network, address string) (net.Listener, error) {
		var listenConfig net.ListenConfig
		return listenConfig.Listen(ctx, network, address)
	}
	return managed, nil
}

// Start 同步绑定监听地址并异步执行 Serve。
// Start 仅允许在 created 状态调用；成功启动后不支持重启。
func (s *Managed) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.state {
	case stateCreated:
	case stateRunning, stateFailed:
		return s.lifecycleError("start", ErrAlreadyStarted)
	case stateStopping, stateStopped:
		return s.lifecycleError("start", ErrStopped)
	}

	listener, err := s.listen(ctx, "tcp", s.addr)
	if err != nil {
		return s.operationError("listen", err)
	}
	s.listener = listener
	s.serveDone = make(chan struct{})
	s.state = stateRunning
	go s.serve(listener)
	return nil
}

// Stop 启动或等待唯一的后台关闭流程。
// ctx 只控制本次等待；取消或超时不会中止已经启动的后台清理。
func (s *Managed) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	if s.state == stateCreated {
		s.state = stateStopped
		close(s.cleanupDone)
		s.mu.Unlock()
		return nil
	}
	if !s.cleanupStarted && (s.state == stateRunning || s.state == stateFailed) {
		s.cleanupStarted = true
		s.state = stateStopping
		go s.cleanup()
	}
	done := s.cleanupDone
	s.mu.Unlock()

	// cleanup 已经完成时稳定返回最终结果，即使调用方 context 同时取消。
	select {
	case <-done:
		return s.finalStopError()
	default:
	}
	select {
	case <-done:
		return s.finalStopError()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// HTTPServer 返回 Managed 拥有的底层 net/http server。
// 返回值的 Handler 是内部 drain tracker，而不是构造时传入的原始 handler。
func (s *Managed) HTTPServer() *http.Server {
	return s.server
}

func (s *Managed) serve(listener net.Listener) {
	err := s.server.Serve(listener)

	s.mu.Lock()
	expected := errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) ||
		((s.state == stateStopping || s.state == stateStopped) && errors.Is(err, context.Canceled))
	var callback func(error)
	var callbackErr error
	if err != nil && !expected {
		callbackErr = s.operationError("serve", err)
		s.serveErr = callbackErr
		if s.state == stateRunning {
			s.state = stateFailed
		}
		callback = s.onServeError
	}
	close(s.serveDone)
	s.mu.Unlock()

	if callback != nil {
		callback(callbackErr)
	}
}

func (s *Managed) cleanup() {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	shutdownErr := s.operationError("shutdown", s.server.Shutdown(cleanupCtx))
	listenerErr := wrapExpectedCloseError(s, "close listener", s.closeListener())
	var closeErr error
	if shutdownErr != nil {
		closeErr = wrapExpectedCloseError(s, "force close", s.server.Close())
	}
	drainErr := s.operationError("wait handlers", s.drain.Wait(cleanupCtx))

	s.mu.Lock()
	serveDone := s.serveDone
	s.mu.Unlock()
	if serveDone != nil {
		<-serveDone
	}

	s.mu.Lock()
	s.stopErr = errors.Join(shutdownErr, listenerErr, closeErr, drainErr, s.serveErr)
	s.state = stateStopped
	close(s.cleanupDone)
	s.mu.Unlock()
}

func (s *Managed) closeListener() error {
	s.mu.Lock()
	listener := s.listener
	s.mu.Unlock()
	if listener == nil {
		return nil
	}
	return listener.Close()
}

func (s *Managed) finalStopError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopErr
}

func (s *Managed) lifecycleError(operation string, err error) error {
	return fmt.Errorf("%s http server %q at %q: %w", operation, s.name, s.addr, err)
}

func (s *Managed) operationError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s http server %q at %q: %w", operation, s.name, s.addr, err)
}

func wrapExpectedCloseError(s *Managed, operation string, err error) error {
	if err == nil || errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return s.operationError(operation, err)
}
