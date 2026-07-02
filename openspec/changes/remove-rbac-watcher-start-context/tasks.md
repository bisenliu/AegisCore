## 1. 代码更新

- [x] 1.1 将 `user-service/internal/features/permission/infrastructure/redis/watcher.go` 中 `Watcher.Start(context.Context)` 改为无参数 `Watcher.Start()`，保留内部 `context.WithCancel(context.Background())` 和 `run(ctx, done)` 关闭语义。
- [x] 1.2 更新 `NewWatcher` 注册的 Fx `OnStart` hook，使其显式丢弃 Fx 启动 context 并调用 `watcher.Start()`。
- [x] 1.3 使用 `rg "Start\\(context.Context\\)|watcher.Start\\(" user-service/internal/features/permission -g '*.go'` 检查并清理旧签名调用点，不保留旧签名兼容包装。

## 2. 测试更新

- [x] 2.1 更新 `user-service/internal/features/permission/infrastructure/redis/watcher_test.go` 中 watcher 启动调用为 `watcher.Start()`。
- [x] 2.2 补充或调整测试，明确取消 Fx 启动 context 不是 watcher 后台循环的生命周期契约，并验证 `Stop(ctx)` 仍能关闭后台循环。
- [x] 2.3 运行 `go test ./user-service/internal/features/permission/infrastructure/redis -run 'TestWatcher'` 验证 watcher 行为。

## 3. 规格与全量验证

- [x] 3.1 运行 `make user-service-architecture-lint` 验证 OpenSpec 和架构规则。
- [x] 3.2 使用 `git diff --check` 检查格式和空白问题，并确认未改动 OpenAPI、migration、部署或观测生成物。
- [x] 3.3 将本次预期代码、测试和 OpenSpec 变更加到暂存区。
- [x] 3.4 运行 `make lint`。
- [x] 3.5 运行 `make verify`。
- [x] 3.6 确认 `openspec status --change "remove-rbac-watcher-start-context"` 显示 change 已满足 apply 要求，并检查最终 diff 只包含本 change 范围。
