## 1. 测试布局与 harness

- [x] 1.1 梳理现有 RBAC policy sync、watcher、loader、engine、user-role cache 和 role/permission 写侧测试位置，确定故障注入测试放置路径。
- [x] 1.2 在测试包内新增 harness，支持控制 loader 阻塞、Redis publish 失败、dispatcher 重试、watcher 消息乱序和 cache loader 延迟。
- [x] 1.3 为 harness 增加 eventually-style 断言工具，统一记录 database revision、applied revision、lag、消息序列和授权结果诊断信息。
- [x] 1.4 确认新增辅助类型只存在于 `_test.go` 或测试 helper 中，不新增 `common`、`internal/shared`、生产 `internal/integration`、`ForTest` 或 `testHook` 正式 API。

## 2. 故障注入验收场景

- [x] 2.1 新增数据库提交后 Redis publish 或 Pub/Sub 失败并恢复的测试，验证没有新 RBAC 写入时所有副本 lag 归零且授权投影收敛。
- [x] 2.2 新增两次 reload 逆序完成测试，验证旧 revision 结果不能覆盖最新 applied revision、Casbin projection 或 user-role cache 状态。
- [x] 2.3 新增 Add/Remove/Replace 同步事件重复投递、乱序投递和 dispatcher 重试测试，验证重放不丢通知且保持幂等收敛。
- [x] 2.4 新增 100 并发 RBAC 写测试，注入 loader 阻塞、watcher 乱序或 cache loader 延迟，验证最终 database revision、applied revision 和授权结果一致。
- [x] 2.5 为每个场景补充 fail-closed 或授权 allow/deny 断言，避免只验证通知调用或 revision 数值导致假收敛。

## 3. 文档和规格同步

- [x] 3.1 更新 `docs/TESTING.md` 或相关测试说明，记录 RBAC policy sync 故障注入场景、风险、预期收敛条件、运行命令和 `AEGISCORE_TEST_CONTAINERS=1` 依赖。
- [x] 3.2 根据实现结果核对 `openspec/changes/add-rbac-policy-sync-fault-injection-tests/specs/rbac-access-control/spec.md`，确保验收场景与测试覆盖一致。
- [x] 3.3 如实施阶段发现测试边界或运行方式变化，同步更新 `design.md` 中的路径、验证方式或未决问题。

## 4. 验证

- [x] 4.1 运行新增故障注入测试所在 package 的 Go 测试并修复失败。
- [x] 4.2 在容器依赖可用时运行 `AEGISCORE_TEST_CONTAINERS=1 make test` 或等价窄化命令，确认 PostgreSQL/Redis 相关验收可执行。
- [x] 4.3 运行 `openspec validate --specs` 和 `make user-service-architecture-lint`，确认 OpenSpec 与架构边界检查通过。
- [x] 4.4 暂存本 change 的预期代码、文档和 OpenSpec 变更后运行 `make lint`。
- [x] 4.5 暂存本 change 的预期代码、文档和 OpenSpec 变更后运行 `make verify`，确认无生成物 drift 或未暂存预期 diff。
