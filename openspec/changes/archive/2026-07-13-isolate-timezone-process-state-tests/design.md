## Context

`common/runtime/timezone` 的 `InitConfig` 会在成功时设置 `TZ`、`time.Local` 和包级初始化状态。该行为本身是进程级一次性初始化，生产代码需要保持“成功后同进程只初始化一次、失败后允许重试”的语义。

当前测试通过 helper 保存 `time.Local` 和 `TZ`，但直接裸写 `state = timezoneState{}`。这让测试绕过了 `timezoneState` 自身的锁，也让后续新增 `t.Parallel` 或复用 helper 时更容易引入数据竞争。

## Goals / Non-Goals

**Goals:**

- 提供包内受控的测试 reset 入口，重置 `state` 时遵守同一把内部锁。
- 在测试 helper 中集中保存和恢复 `TZ`、`time.Local`、timezone 初始化状态。
- 用注释明确 timezone 测试依赖进程级全局状态，不能无隔离并行化。
- 通过 `go test -race ./runtime/timezone` 验证 race 隔离。

**Non-Goals:**

- 不改变 `InitConfig` 的生产语义。
- 不改变默认时区、配置字段或 Fx module 行为。
- 不引入 request-scoped、service-scoped 或可重复生产初始化机制。
- 不新增跨包公开 API，仅为包内测试提供受控 reset。

## Decisions

1. 使用包内 `resetForTest` helper，而不是测试中直接赋值 `state`。

   原因：reset 需要持有 `state.mu` 后再清空 `initialized`，与 `init` 的并发语义保持一致。备选方案是在测试中继续裸写 `state = timezoneState{}`，但该方式绕过锁且不表达进程级状态约束。

2. 测试 helper 统一管理 `TZ`、`time.Local` 和 timezone state。

   原因：这些状态必须作为同一个隔离单元保存和恢复，避免单个测试失败或新增测试时遗漏恢复。备选方案是在各测试中分别恢复环境变量和 `time.Local`，但重复逻辑更容易漂移。

3. 相关测试保持串行，不使用 `t.Parallel`。

   原因：`TZ` 与 `time.Local` 是进程级全局状态，Go test 的并行测试无法在不加全局测试锁的情况下安全共享。当前测试数量少，显式串行比引入额外全局测试锁更直接。

## Risks / Trade-offs

- 进程级状态仍然无法与其他并行测试完全隔离 → timezone 包内测试不使用 `t.Parallel`，并在 helper 注释中明确约束。
- reset helper 仅服务包内测试 → 使用未导出的 helper，避免对生产调用方形成可依赖 API。
- `t.Setenv` 与手动 `time.Local` 恢复存在 cleanup 顺序要求 → helper 统一注册 cleanup，保证状态恢复顺序可读且集中维护。

## Migration Plan

- 更新 OpenSpec delta 后实施代码改动。
- 修改 `common/runtime/timezone` 包内测试 helper 和必要注释。
- 在 `common` 模块执行 `go test -race ./runtime/timezone`。
- 如需回滚，恢复本 change 中的两个 timezone 文件和 OpenSpec change 目录即可；无数据、部署或 API 迁移。

## Open Questions

无。
