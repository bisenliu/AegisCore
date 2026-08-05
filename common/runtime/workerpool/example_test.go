package workerpool_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/workerpool"
)

// ExamplePool 展示任务池的基本生命周期：创建、提交、等待任务完成，并由资源拥有者显式停止。
// Submit 返回 nil 只代表任务已被接收；业务代码不能把它当成 Task.Run 已经成功完成。
func ExamplePool() {
	pool, err := workerpool.New(zap.NewNop(), workerpool.Options{
		Name:    "document.thumbnail",
		Workers: 1,
	})
	if err != nil {
		panic(err)
	}

	done := make(chan struct{})
	err = pool.Submit(context.Background(), workerpool.Task{
		Name: "generate_thumbnail",
		Run: func(context.Context) error {
			close(done)
			return nil
		},
	})
	if err != nil {
		panic(err)
	}

	// 实际应用通常由业务状态或生命周期协调完成；示例使用 channel 明确观察任务完成。
	<-done
	if err := pool.Stop(context.Background()); err != nil {
		panic(err)
	}
	fmt.Println(pool.Stats().Completed)

	// Output: 1
}

// ExamplePool_Submit_requestContext 展示任务跟随请求 context 取消。
// Task.Run 必须协作检查 context，并把收到的 context 继续传给数据库、Redis 或 HTTP 调用。
func ExamplePool_Submit_requestContext() {
	pool, err := workerpool.New(zap.NewNop(), workerpool.Options{Name: "document.preview", Workers: 1})
	if err != nil {
		panic(err)
	}

	requestCtx, cancelRequest := context.WithCancel(context.Background())
	taskResult := make(chan error, 1)
	err = pool.Submit(requestCtx, workerpool.Task{
		Name: "rebuild_preview",
		Run: func(taskCtx context.Context) error {
			<-taskCtx.Done()
			taskResult <- taskCtx.Err()
			return taskCtx.Err()
		},
	})
	if err != nil {
		panic(err)
	}

	cancelRequest()
	observedErr := <-taskResult
	if err := pool.Stop(context.Background()); err != nil {
		panic(err)
	}
	fmt.Println(errors.Is(observedErr, context.Canceled))
	fmt.Println(pool.Stats().Failed)

	// Output:
	// true
	// 1
}

// ExamplePool_Submit_detachedContext 展示任务需要在请求结束后继续运行时的 context 所有权。
// context.WithoutCancel 会移除请求取消和 deadline，但保留 trace 等 value；任务应再设置自己的 deadline。
func ExamplePool_Submit_detachedContext() {
	type traceKey struct{}

	pool, err := workerpool.New(zap.NewNop(), workerpool.Options{Name: "auth.session_purge", Workers: 1})
	if err != nil {
		panic(err)
	}

	requestCtx, cancelRequest := context.WithCancel(context.WithValue(context.Background(), traceKey{}, "trace-123"))
	backgroundCtx := context.WithoutCancel(requestCtx)
	cancelRequest()

	result := make(chan bool, 1)
	err = pool.Submit(backgroundCtx, workerpool.Task{
		Name: "purge_expired_sessions",
		Run: func(poolCtx context.Context) error {
			taskCtx, cancel := context.WithTimeout(poolCtx, time.Second)
			defer cancel()
			result <- taskCtx.Err() == nil && taskCtx.Value(traceKey{}) == "trace-123"
			return nil
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(<-result)
	if err := pool.Stop(context.Background()); err != nil {
		panic(err)
	}

	// Output: true
}

// ExamplePool_Submit_backpressure 展示固定 worker 全忙时的同步背压。
// 当前 Pool 没有独立 queue capacity；后续 Submit 会阻塞，不适合要求立即返回的入口。
func ExamplePool_Submit_backpressure() {
	pool, err := workerpool.New(zap.NewNop(), workerpool.Options{Name: "document.render", Workers: 1})
	if err != nil {
		panic(err)
	}

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	err = pool.Submit(context.Background(), workerpool.Task{
		Name: "first_render",
		Run: func(context.Context) error {
			close(firstStarted)
			<-releaseFirst
			return nil
		},
	})
	if err != nil {
		panic(err)
	}
	<-firstStarted

	secondSubmit := make(chan error, 1)
	go func() {
		secondSubmit <- pool.Submit(context.Background(), workerpool.Task{
			Name: "second_render",
			Run:  func(context.Context) error { return nil },
		})
	}()

	// Waiting 来自 ants，明确观察第二个 Submit 已进入背压等待，而不是依赖固定 Sleep 猜测时序。
	waitForExampleCondition(func() bool { return pool.Stats().Waiting == 1 })
	fmt.Println(pool.Stats().Waiting)
	close(releaseFirst)
	fmt.Println(<-secondSubmit == nil)
	if err := pool.Stop(context.Background()); err != nil {
		panic(err)
	}

	// Output:
	// 1
	// true
}

// ExamplePool_Stop_retryDrain 展示第一次 Stop 被调用方 context 取消后，后续 Stop 继续等待同一份 drain。
// Stop 不会强杀忽略 context 的任务，因此任务本身仍必须遵守取消协议。
func ExamplePool_Stop_retryDrain() {
	pool, err := workerpool.New(zap.NewNop(), workerpool.Options{Name: "document.cleanup", Workers: 1})
	if err != nil {
		panic(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	err = pool.Submit(context.Background(), workerpool.Task{
		Name: "cleanup",
		Run: func(context.Context) error {
			close(started)
			<-release // 示例故意模拟一个暂时忽略取消的任务。
			return nil
		},
	})
	if err != nil {
		panic(err)
	}
	<-started

	firstStopCtx, cancelFirstStop := context.WithCancel(context.Background())
	cancelFirstStop()
	firstErr := pool.Stop(firstStopCtx)
	fmt.Println(errors.Is(firstErr, context.Canceled))

	close(release)
	secondErr := pool.Stop(context.Background())
	fmt.Println(secondErr == nil)

	// Output:
	// true
	// true
}

// ExamplePool_Stats 展示累计任务计数与瞬时运行状态。
// Stats 的字段分别读取，高并发变化期间不保证来自同一个精确时间点。
func ExamplePool_Stats() {
	pool, err := workerpool.New(zap.NewNop(), workerpool.Options{Name: "document.index", Workers: 1})
	if err != nil {
		panic(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	err = pool.Submit(context.Background(), workerpool.Task{
		Name: "index_document",
		Run: func(context.Context) error {
			close(started)
			<-release
			return nil
		},
	})
	if err != nil {
		panic(err)
	}
	<-started

	running := pool.Stats()
	fmt.Println(running.Submitted, running.Started, running.Running)
	close(release)
	if err := pool.Stop(context.Background()); err != nil {
		panic(err)
	}
	stopped := pool.Stats()
	fmt.Println(stopped.Completed, stopped.Closed)

	// Output:
	// 1 1 1
	// 1 true
}

func waitForExampleCondition(condition func() bool) {
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			panic("timed out waiting for example condition")
		}
		runtime.Gosched()
	}
}
