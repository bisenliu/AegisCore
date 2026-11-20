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
	// stateCreated 表示 HTTP 服务实例已创建，尚未开始绑定端口监听。
	stateCreated serverState = iota
	// stateRunning 表示端口监听成功且正在后台提供服务。
	stateRunning
	// stateFailed 表示启动监听或后台服务遇到不可恢复的异常。
	stateFailed
	// stateStopping 表示正在执行优雅停机和连接排出。
	stateStopping
	// stateStopped 表示服务已完全停止并清理完所有资源。
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
	if s.canStartCleanupLocked() {
		s.cleanupStarted = true
		s.state = stateStopping
		// cleanup 必须在调用方取消后继续完成，不能复用 Stop 的等待 context。
		go s.cleanup() // #nosec G118 -- 后台清理使用独立超时，不能绑定调用方 context。
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

// canStartCleanupLocked 检查当前服务是否允许开始执行关闭收尾流程，防止重复触发关闭。
func (s *Managed) canStartCleanupLocked() bool {
	// cleanup 只从 running/failed 进入 stopping；created 直接 stopped，
	// stopping/stopped 只等待同一个 cleanupDone 结果。
	return !s.cleanupStarted && (s.state == stateRunning || s.state == stateFailed)
}

// HTTPServer 返回 Managed 拥有的底层 net/http server。
// 返回值的 Handler 是内部 drain tracker，而不是构造时传入的原始 handler。
func (s *Managed) HTTPServer() *http.Server {
	return s.server
}

// serve 在后台持续监听并接收外部访问请求；如果是系统正常关闭导致的停止监听，则视为正常退出而非系统故障。
func (s *Managed) serve(listener net.Listener) {
	err := s.server.Serve(listener)

	s.mu.Lock()
	var callback func(error)
	var callbackErr error
	if err != nil && !s.isExpectedServeErrorLocked(err) {
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

// isExpectedServeErrorLocked 判断网络监听停止的原因是否为系统主动下线；主动下线属于正常行为，不视为程序异常。
func (s *Managed) isExpectedServeErrorLocked(err error) bool {
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return true
	}
	// Stop 关闭 listener 时，部分自定义 listener 会以 context.Canceled 退出 Accept。
	return (s.state == stateStopping || s.state == stateStopped) && errors.Is(err, context.Canceled)
}

// cleanup 执行系统停止时的资源清理工作，包括关闭监听网络端口并等待正在处理的请求全部做完。
func (s *Managed) cleanup() {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	shutdownErr := s.shutdown(cleanupCtx)
	listenerErr := s.closeManagedListener()
	closeErr := s.forceCloseAfterShutdownError(shutdownErr)
	drainErr := s.waitHandlers(cleanupCtx)
	s.waitServeDone()

	s.mu.Lock()
	s.stopErr = errors.Join(shutdownErr, listenerErr, closeErr, drainErr, s.serveErr)
	s.state = stateStopped
	close(s.cleanupDone)
	s.mu.Unlock()
}

// shutdown 停止接收新的网络请求，并给当前正在处理中的请求预留平稳退出的缓冲时间。
func (s *Managed) shutdown(ctx context.Context) error {
	return s.operationError("shutdown", s.server.Shutdown(ctx))
}

// closeManagedListener 关闭系统占用的网络监听端口，停止接受外界新的连接。
func (s *Managed) closeManagedListener() error {
	return wrapExpectedCloseError(s, "close listener", s.closeListener())
}

// forceCloseAfterShutdownError 当平稳关机等待超时后，强制切断所有网络连接以确保系统能正常停止，同时保留原始的超时错误记录。
func (s *Managed) forceCloseAfterShutdownError(shutdownErr error) error {
	if shutdownErr == nil {
		return nil
	}
	return wrapExpectedCloseError(s, "force close", s.server.Close())
}

// waitHandlers 等待所有业务处理协程将手头的请求完成；如果等待超时则返回错误。
func (s *Managed) waitHandlers(ctx context.Context) error {
	return s.operationError("wait handlers", s.drain.Wait(ctx))
}

// waitServeDone 等待负责接收网络连接的后台任务彻底退出。
func (s *Managed) waitServeDone() {
	s.mu.Lock()
	serveDone := s.serveDone
	s.mu.Unlock()
	if serveDone != nil {
		<-serveDone
	}
}

// closeListener 关闭底层的网络监听通道；如果已经关闭过则不再重复操作。
func (s *Managed) closeListener() error {
	s.mu.Lock()
	listener := s.listener
	s.mu.Unlock()
	if listener == nil {
		return nil
	}
	return listener.Close()
}

// finalStopError 记录服务关闭时遇到的第一个错误，避免后续连锁收尾问题掩盖最开始的根本故障原因。
func (s *Managed) finalStopError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopErr
}

// lifecycleError 在服务启动或停止出错时，补充服务名称和操作步骤信息，方便日志排查。
func (s *Managed) lifecycleError(operation string, err error) error {
	return fmt.Errorf("%s http server %q at %q: %w", operation, s.name, s.addr, err)
}

// operationError 给操作执行失败补充服务标识，方便排查是哪个组件在什么环节出错。
func (s *Managed) operationError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s http server %q at %q: %w", operation, s.name, s.addr, err)
}

// wrapExpectedCloseError 识别并过滤因主动关闭网络端口而产生的常规网络中断提示，避免将正常的关机操作误报为系统故障。
func wrapExpectedCloseError(s *Managed, operation string, err error) error {
	if err == nil || errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return s.operationError(operation, err)
}
