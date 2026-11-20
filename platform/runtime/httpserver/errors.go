package httpserver

import "errors"

var (
	// ErrInvalidOptions 表示 HTTP server 构造参数不合法。
	ErrInvalidOptions = errors.New("invalid http server options")
	// ErrAlreadyStarted 表示 HTTP server 已经启动或曾经启动失败。
	ErrAlreadyStarted = errors.New("http server already started")
	// ErrStopped 表示 HTTP server 已进入停止流程，不能重新启动。
	ErrStopped = errors.New("http server has stopped")
)
