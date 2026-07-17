## 1. Workerpool API 解耦

- [x] 1.1 修改 `common/runtime/workerpool.New` 签名为普通 Go 构造器，删除 `fx.Lifecycle` 参数、`fx.Hook` 注册和 `go.uber.org/fx` 导入。
- [x] 1.2 更新 `common/runtime/workerpool` 构造器、`Pool` 和 `Stop` 相关注释，明确调用方拥有显式 `Stop(ctx)` 责任。
- [x] 1.3 搜索并移除 `common/runtime/workerpool` 内所有 `go.uber.org/fx`、`fx.` 和 `fxtest` 生产与测试依赖。

## 2. Auth 调用点迁移

- [x] 2.1 修改 `user-service/internal/features/auth/infrastructure/redis/session_purge_pool.go`，使用新 `workerpool.New` 创建任务池。
- [x] 2.2 在 auth Redis 服务私有 Fx composition 中显式登记 `pool.Stop(ctx)`，保持 purge pool stop hook 先于 Redis client stop hook。
- [x] 2.3 确认未修改 session purge 业务语义、Redis key、任务内容、worker 数、StopTimeout、错误语义或指标语义。

## 3. 测试更新

- [x] 3.1 重写 `common/runtime/workerpool` 测试，删除 `fxtest` fixture，覆盖构造失败、关闭后拒绝、panic recovery、StopTimeout、重复 Stop 和完整 drain。
- [x] 3.2 更新 auth Redis infrastructure 测试，确保 `NewSessionPurgePool` 适配新 workerpool 构造器且关闭顺序测试仍验证 purge pool 在 Redis stop 前 drain。
- [x] 3.3 运行 `rg -n 'go\.uber\.org/fx|fx\.|fxtest' common/runtime/workerpool` 并确认无输出。

## 4. 规格与架构验证

- [x] 4.1 运行 `openspec validate detach-workerpool-from-fx-lifecycle` 并修复 proposal、design、specs 或 tasks 的结构问题。
- [x] 4.2 运行 `make user-service-architecture-lint` 并修复架构边界问题。

## 5. Package 验证

- [x] 5.1 运行 `cd common && go test ./runtime/workerpool -count=1`，确认 workerpool 行为测试通过。
- [x] 5.2 运行 `cd user-service && go test ./internal/features/auth/infrastructure/redis -count=1`，确认 auth Redis session purge pool 迁移通过。

## 6. 全量交付验证

- [x] 6.1 检查 `git diff`，确认仅包含本 change 预期的代码、测试和 OpenSpec artifact 变更，且没有生成物漂移。
- [x] 6.2 暂存本 change 预期变更后运行 `make lint` 并修复失败项。
- [x] 6.3 在预期变更已暂存状态下运行 `make verify` 并修复失败项。
- [x] 6.4 所有实现、规格、测试和验证完成后，将对应 checkbox 更新为 `- [x]`。
