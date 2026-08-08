## 1. 实现准备

- [X] 1.1 阅读 `user-service/internal/features/auth/infrastructure/redis/refresh_session_store.go`、相关 session purge 测试和 `common/runtime/workerpool` 日志实现，确认当前任务字段和日志捕获测试入口。
- [X] 1.2 确定日志字段最小集合：保留稳定任务名、`batch_size`、`cut_time`，按需加入不可逆 opaque 标识；不得保留 `purge_key`、`session_prefix`、明文 `user_id` 或等价兼容字段。

## 2. Auth purge 日志字段收敛

- [X] 2.1 修改 `DeleteAllUserSessions` 中提交给 workerpool 的 `Task.Fields`，删除完整 Redis key、Redis key prefix、明文用户标识和可拼装 session key material。
- [X] 2.2 确认 `purgeKey` 和 `sessionPrefix` 只作为后台清理闭包内部参数继续使用，保持 `purgeDetachedUserSessions` 的 Redis 清理语义不变。
- [X] 2.3 如实现不可逆 opaque 标识，将其放在 auth Redis infrastructure 私有边界内，禁止引入 common 业务专用脱敏 helper 或兼容分支。

## 3. 测试覆盖

- [X] 3.1 增加或更新 auth Redis session purge 失败路径测试，注入后台 purge error，捕获 workerpool `worker pool task failed` 日志，并断言日志字段名和值不包含 `purge_key`、`session_prefix`、Redis namespace、`auth:session`、`auth:user:sessions`、`{user_uuid}`、session ID 或可拼装 session key material。
- [X] 3.2 增加或更新 auth Redis session purge panic 路径测试，捕获 workerpool `worker pool task panicked` 日志，并断言 panic 与 stacktrace 可观察但不包含 Redis key material。
- [X] 3.3 增加或保留成功 purge 行为测试，确认日志字段收敛不改变退出全部会话后的 session 物理清理语义。

## 4. 验证与收尾

- [X] 4.1 运行 `go test ./user-service/internal/features/auth/infrastructure/redis ./common/runtime/workerpool`，修复所有失败。
- [X] 4.2 运行 `make user-service-architecture-lint`，确认 OpenSpec change 和架构边界检查通过。
- [X] 4.3 检查生成物和非预期改动，确认本 change 不修改 OpenAPI、Ent 生成物、Atlas migration 或部署资产。
- [X] 4.4 将本次预期代码、测试和 OpenSpec artifact 变更加到暂存区。
- [X] 4.5 运行 `make lint`，修复所有失败。
- [X] 4.6 运行 `make verify`，修复所有失败，并确认最终 diff 状态符合仓库验证要求。
