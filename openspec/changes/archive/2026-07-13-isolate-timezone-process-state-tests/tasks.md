## 1. OpenSpec 与实现准备

- [x] 1.1 确认 `openspec/changes/isolate-timezone-process-state-tests/` 包含 `proposal.md`、`design.md`、`tasks.md` 和 `specs/shared-platform-primitives/spec.md`
- [x] 1.2 阅读 `common/runtime/timezone` 当前实现与测试，确认 `state`、`TZ` 和 `time.Local` 的进程级状态边界

## 2. Timezone 测试隔离实现

- [x] 2.1 在 `common/runtime/timezone` 包内引入受控测试 reset helper，重置初始化状态时持有 `timezoneState` 内部锁
- [x] 2.2 重构 `timezone_test.go` 的测试隔离 helper，集中保存并恢复 `TZ`、`time.Local` 和 timezone 初始化状态
- [x] 2.3 添加或调整注释，明确 timezone 初始化和相关测试依赖进程级全局状态，测试不得无隔离并行化

## 3. 验证

- [x] 3.1 在 `common` 模块执行 `go test -race ./runtime/timezone`
- [x] 3.2 执行 `make user-service-architecture-lint` 验证 OpenSpec 与架构约束
- [x] 3.3 检查本次相关路径 diff，确认未修改无关生成物或超出 change 范围的文件
