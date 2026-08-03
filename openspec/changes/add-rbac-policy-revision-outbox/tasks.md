## 1. 数据模型与迁移

- [x] 1.1 在 `user-service/internal/persistence/ent/schema/` 定义 RBAC policy revision 与 outbox event schema，包含 revision identity、事件 kind/reason、对象 UUID、状态、重试、幂等键、时间字段、Ent 一对一关系和 dispatcher 扫描索引，并补充 schema 字段与约束测试；数据库 migration 遵循仓库全局无真实外键策略。
- [x] 1.2 运行 `make user-service-generate` 更新 Ent 生成代码，审查仅包含新 schema 的预期生成物；再次运行同一命令并用 `git diff --exit-code -- user-service/internal/persistence/ent` 检查生成 drift。
- [x] 1.3 运行 `make user-service-migrate-diff name=add-rbac-policy-revision-outbox` 生成 Atlas SQL migration，审查 identity/sequence、表、唯一约束和索引，确认遵循全局无真实外键策略、不回填 Redis version 且可在应用 rollout 前独立执行。
- [x] 1.4 运行 `make user-service-migrate-validate`，并按仓库 migration 生成流程重新检查无未预期 schema drift。

## 2. 写侧事务与 outbox

- [x] 2.1 在 role application 消费侧定义最小 policy change 元数据与已提交 revision 写结果，覆盖 `policy_changed`、`user_role_changed`、稳定 reason 和相关对象 ID，保持 application/domain 不依赖 Ent、SQL、Redis 或 permission infrastructure。
- [x] 2.2 在 `user-service/internal/features/role/infrastructure/postgres/` 实现共享的 transaction 内 revision/outbox append 流程，统一处理 begin、业务写、revision 插入、outbox 插入、commit 和 rollback 错误，且 transaction 内不执行外部 I/O。
- [x] 2.3 改造角色创建、更新和启停 store，使实际 mutation、revision 和 pending outbox 原子提交；校验失败、not-found、系统角色保护或未发生 mutation 时不得分配 revision。
- [x] 2.4 改造角色权限添加、替换和删除 store，使完整替换的删除/插入、revision 和 pending outbox 位于同一 transaction，并返回已提交数据库 revision。
- [x] 2.5 改造用户角色添加、替换和删除 store，使完整替换的删除/插入、revision 和定向 pending outbox 位于同一 transaction，并返回已提交数据库 revision。
- [x] 2.6 更新 role command service 与 Fx composition 接线，使用 store 返回的 revision 调用必需 notifier；删除 mutation 提交后再创建可靠性事实的独立路径，不增加 no-op、nil fallback 或 Redis 权威兼容分支。

## 3. 数据库 revision 驱动的即时同步

- [x] 3.1 调整 permission application 的 `PolicyChangeNotifier`/coordinator 契约，使全局 policy 变更和定向 user-role 变更均接收已提交数据库 revision，并保留本地 reload、缓存失效和组合错误语义。
- [x] 3.2 调整 Redis policy version adapter，仅以原子 max 语义缓存调用方传入的数据库 revision并发布现有 Pub/Sub 通知；移除 `INCR`、时间戳或本地 counter 分配权威版本的代码与测试假设。
- [x] 3.3 更新 watcher/version tracker 的必要接线与单元测试，保持现有消息协议和周期性补偿行为，并证明较小 revision 不覆盖或倒退已知较大 revision。
- [x] 3.4 更新相关中文代码注释和英文 log message/稳定 `snake_case` 字段，使同步错误可记录 reason 与 revision，且不记录 SQL、Redis key 或原始 event payload。

## 4. 原子性与失败测试

- [x] 4.1 为角色、角色权限和用户角色全部在线 mutation 增加成功测试，验证每次提交同时产生唯一单调 revision、对应 pending outbox、零重试次数、稳定幂等键和正确对象 ID。
- [x] 4.2 增加业务 mutation 失败与 revision 插入失败测试，验证业务、revision、outbox 全部回滚且 notifier 未调用。
- [x] 4.3 增加 outbox 插入失败与 commit 失败测试，验证业务、revision、outbox 全部不可见、rollback 错误得到保留且 notifier 未调用。
- [x] 4.4 增加即时 reload、缓存失效、Redis 写入和 Pub/Sub 发布失败测试，验证 API 保留同步错误且已提交 revision/pending outbox 不被删除、回滚或误标记为 delivered。
- [x] 4.5 运行 role 与 permission 相关 Go package 测试及 PostgreSQL transaction 集成测试，修复失败并记录使用的精确测试命令。

## 5. 规格与交付门禁

- [x] 5.1 对照 `openspec/changes/add-rbac-policy-revision-outbox/specs/rbac-access-control/spec.md` 检查实现，确认未实现 dispatcher、revision-aware Casbin reload、Redis Pub/Sub 协议改造、lag 指标或旧 Redis counter 权威兼容分支。
- [x] 5.2 运行 `make user-service-architecture-lint`，确认 outbox 归属 role PostgreSQL infrastructure，且未向 `common/`、`internal/shared/`、`internal/integration/` 或 application/domain 泄漏 concrete persistence 依赖。
- [x] 5.3 重新运行 `make user-service-generate` 和 `make user-service-migrate-validate`，使用 `git diff --exit-code` 检查生成物与 migration 无 drift，并确认本 change 未产生 OpenAPI、部署清单或观测资产的非预期 diff。
- [x] 5.4 在全部实现、测试、规格和生成物完成后，仅暂存本 change 的预期代码、migration、生成物和 OpenSpec artifacts，检查 `git status` 与 staged diff 不包含无关或敏感文件。
- [x] 5.5 在预期变更已暂存后运行 `make lint`；只有命令通过后才完成本任务。
- [x] 5.6 在预期变更已暂存后运行 `make verify`；只有全部验证和最终 drift 检查通过后才将 change 标记为实现完成。
