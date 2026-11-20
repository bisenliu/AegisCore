package workerpool

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// Task 描述一个后台任务单元。
type Task struct {
	Name   string
	Fields []zap.Field
	Run    func(context.Context) error
}

// Options 配置后台任务池。
type Options struct {
	Name        string
	Workers     int
	StopTimeout time.Duration
}
