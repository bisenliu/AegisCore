## 1. Outbox claim schema 与迁移

- [x] 1.1 扩展 `user-service/internal/persistence/ent/schema/rbacpolicyoutboxevent.go`，增加 `processing`/`failed` 状态、nullable claim token、claim lease 截止时间与并发 claim 扫描索引，并更新 schema 约束测试。
- [x] 1.2 运行 `make user-service-generate` 更新 Ent 生成代码，审查只包含预期 schema 变化；再次运行同一生成命令确认输出稳定，并检查 `git diff --stat -- user-service/internal/persistence/ent` 仅包含预期 schema 生成物。
- [x] 1.3 运行 `make user-service-migrate-diff name=add-rbac-policy-outbox-dispatcher` 生成 additive Atlas SQL migration，审查状态约束、nullable claim 字段和扫描索引，确认不改变既有 mutation/outbox transaction 模型且不新增真实外键。
- [x] 1.4 运行 `make user-service-migrate-validate`，并按仓库迁移流程检查 migration 与 Ent schema 无未预期 drift。

## 2. Dispatcher application 与 PostgreSQL store

- [x] 2.1 在 permission application 定义最小 outbox event、claim、store、publisher、settings、clock/backoff 和只读 status contracts，保持 application 不依赖 Ent、SQL、Redis、Fx 或 role infrastructure。
- [x] 2.2 实现 dispatcher 构造、单轮 dispatch 与后台 poll loop，按 revision 处理 claimed batch，并保证 Start/Stop 幂等、context 取消可终止 in-flight 工作且单轮错误不会使循环静默退出。
- [x] 2.3 在 permission PostgreSQL infrastructure 实现短 transaction claim，使用 `FOR UPDATE SKIP LOCKED` 按 revision 升序选择 due pending/failed 与 lease 过期 processing event，并原子写入 processing、随机 claim token 和 lease 截止时间。
- [x] 2.4 实现条件 ack 与失败记录，要求 event、processing 状态和 claim token 同时匹配；成功时设置 delivered 并清除 claim/error，失败时递增 attempt、截断安全错误摘要、设置 failed/next attempt 并清除 claim。
- [x] 2.5 实现防溢出的有界指数 retry backoff 和只读 backlog/status 查询，确保 failed event 无固定最大 attempt、坏 event 不阻塞后续 due event，健康查询不执行 claim 或 mutation。

## 3. Redis revision 消息与同步语义

- [x] 3.1 定义并实现版本化 Redis policy refresh envelope，包含 schema version、event ID、idempotency key、数据库 policy revision、kind、reason、publisher instance ID 和可选对象 UUID，并严格拒绝缺失字段、未知版本/kind 与非法 UUID。
- [x] 3.2 调整 Redis publisher，使 dispatcher 先以原子 max 语义缓存数据库 revision 再发布新 envelope；任一步失败均返回错误供 outbox 重试，删除旧 payload 与 Redis counter fallback。
- [x] 3.3 调整 watcher 对重复和乱序消息的消费：每条 `policy_changed` 安全执行当前投影 reload，每条 `user_role_changed` 失效指定用户缓存，副作用完成后 tracker 仅按 max 推进且不得仅按 revision gate 跳过消息。
- [x] 3.4 调整在线 role command 的写后同步，使本实例仍立即 reload 或失效相关缓存，但跨副本 Redis publish 只由 dispatcher 承担；Redis 故障不得要求调用方重新提交已持久化 mutation。

## 4. 配置、composition 与 lifecycle

- [x] 4.1 在 user-service 私有配置中增加 dispatcher `poll_interval`、`batch_size`、`claim_timeout`、`retry_backoff.initial` 和 `retry_backoff.max` 默认值、环境覆盖与正值/顺序校验测试。
- [x] 4.2 在 permission composition 中使用 named `primary_db` 与 `cache_redis` 构造 store、publisher 和单一 dispatcher 实例，并通过 runtime 聚合对象暴露稳定 runner/status 视图，不向父 module 暴露 infrastructure concrete type。
- [x] 4.3 更新 RBAC Fx lifecycle 启停顺序与回滚逻辑，显式启动/停止 dispatcher，在 stop context 总预算内等待退出并证明 dispatcher 不关闭共享 Ent/PostgreSQL/Redis resource。
- [x] 4.4 增加 lifecycle 与构图测试，覆盖 constructor 无副作用、重复 Start/Stop、启动回滚、停止超时、循环意外退出状态和缺少必需依赖时拒绝装配。

## 5. Metrics、日志与 health/readiness

- [x] 5.1 扩展 permission feature metrics 与 no-op 实现，记录固定 result/reason/kind 下的 claim、publish、ack、failure/retry、due backlog、最老未完成 event age 和 loop 状态，并增加 collector/低基数标签测试。
- [x] 5.2 增加 dispatcher 英文结构化日志与稳定 `snake_case` 字段，覆盖 claim、成功、退避、lease 冲突和循环状态；测试或审查确认不记录完整 payload、SQL、Redis key、secret 或高基数 ID label。
- [x] 5.3 通过 permission public status contract 将 dispatcher 只读状态接入 `/readyz` 与 `/startupz`，覆盖未启动、循环退出、状态查询失败、可重试 publish 失败和 backlog 的健康语义，且探测不得修改 event。
- [x] 5.4 本 change 未新增 Prometheus alert、Grafana panel 或 runbook 查询；dispatcher feature metrics 由现有 metrics endpoint 导出，因此现有 dashboard/provisioning 资产无需变化。

## 6. 故障恢复与并发验证

- [x] 6.1 增加 PostgreSQL store 集成测试，覆盖多 dispatcher 并发 claim 不重叠、锁行被跳过、lease 过期重领、stale token ack/failure 被拒绝及 delivered event 不再扫描。
- [x] 6.2 增加 Redis 不可用、revision cache 更新失败和 Pub/Sub publish 失败测试，验证 event 保持 failed/pending、attempt 与 next attempt 正确，并在 Redis 恢复后无需新 mutation 自动 delivered。
- [x] 6.3 增加进程重启与 publish 成功但 ack 前崩溃测试，验证 processing event 在 lease 到期后恢复、允许重复通知且 watcher 的 reload/cache invalidation 与 tracker max 语义保持幂等。
- [x] 6.4 增加坏 payload、重复和乱序事件测试，验证坏 event 保留退避且不阻塞 batch，旧协议无 fallback，较旧定向事件不会因较新 revision 已应用而漏掉用户缓存失效。
- [x] 6.5 已运行 `make test`、`go test -race ./internal/features/permission/application ./internal/features/permission/application/authorization ./internal/features/permission/infrastructure/redis ./internal/features/permission/infrastructure/postgres ./internal/features/permission`、相关 role/config/providers/router/bootstrap 测试，以及 `go test ./internal/features/permission/infrastructure/postgres -args -aegiscore.testcontainers`，全部通过。

## 7. 规格与交付门禁

- [x] 7.1 对照 `specs/rbac-access-control/spec.md` 与 `specs/runtime-observability/spec.md` 检查实现，确认未改变 mutation transaction 模型、未实现 revision-aware Casbin reload 或 cache generation，且未保留旧 `INCR` counter/消息 fallback。
- [x] 7.2 运行 `make user-service-architecture-lint`，确认 dispatcher 归属 permission 消费侧、role 写侧无 Redis 依赖，且 RBAC event/store 未下沉到 `common/`、`internal/shared/`、`internal/integration/` 或 application/domain concrete dependency。
- [x] 7.3 重新运行 `make user-service-generate` 与 `make user-service-migrate-validate`，检查生成物和 migration 无 drift，并确认 `make user-service-openapi-generate` 未产生非预期 HTTP/OpenAPI 生成物变化。
- [x] 7.4 在全部实现、测试、规格和生成物完成后，仅暂存本 change 的预期代码、migration、生成物、配置和 OpenSpec artifacts，检查 `git status` 与 staged diff 不包含无关或敏感文件。
- [x] 7.5 在预期变更已暂存后运行 `make lint`；common 与 user-service 均为 `0 issues`。
- [x] 7.6 在预期变更已暂存后运行 `make verify`；lint、architecture lint、generate、全仓测试、OpenAPI 生成与最终 staged drift 检查全部通过。
