package workerpool

import "errors"

var (
	// ErrClosed 表示任务池已停止接收新任务。
	ErrClosed = errors.New("worker pool closed")
	// ErrQueueFull 表示任务池容量已满，当前任务未被接收。
	ErrQueueFull = errors.New("worker pool queue full")
	// ErrInvalidTask 表示提交的任务缺少必需字段。
	ErrInvalidTask = errors.New("worker pool task is invalid")
)
