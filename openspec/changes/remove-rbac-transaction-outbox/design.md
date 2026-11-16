## Context

在线 RBAC 写入目前把业务 mutation、revision counter、revision 和 outbox event 放在同一 PostgreSQL transaction。dispatcher 再 claim event、发布 Redis，并维护 ack、失败退避和 lease。watcher 已经以数据库 policy revision 为权威补偿 Casbin；user-role 的精确补偿则依赖 outbox 重放 `user_id`。

底座只需要稳定的授权事实和可诊断的最终收敛，不需要通用可靠投递框架。

## Goals / Non-Goals

**Goals:**

- 删除 outbox schema、写入、dispatcher、配置、健康检查和指标。
- 保留 policy revision 与 user-role revision 的事务原子性和提交顺序。
- 用数据库 revision 水位补偿丢失的 Pub/Sub，包括 user-role 缓存失效。
- 保持提交后的本地同步失败不回滚已提交 mutation。

**Non-Goals:**

- 不把 outbox 下沉到 `common/`，也不引入 MQ、eventbus 或 Redis Streams。
- 不改变 HTTP API、权限目录、Casbin subject/route/method 或认证会话。
- 不承诺 Pub/Sub 至少一次或 exactly-once。

## Decisions

1. 事务只提交业务事实和 revision。提交后 coordinator 完成本实例同步，再由 Redis publisher best-effort 发布 hint；发布失败只记录日志，不创建待重试事件。
2. user-role 补偿读取 `rbac_user_role_revisions` 中高于本实例已观察水位的行，按 `user_id` 失效；查询失败或水位跨度超过有界批次时清空全部用户角色缓存，避免漏失效。
3. policy 补偿继续只比较数据库 latest policy revision 与 engine applied revision。user-role revision 不得改变 policy lag 或触发 Casbin reload。
4. 通过 Ent schema 删除 outbox，并生成 Atlas migration 删除表、edge 和索引。不手改 Ent 生成代码。

备选方案是保留 outbox 仅服务 user-role。它仍会保留 claim、lease、状态和后台循环，不符合底座边界，因此不采用。

## Risks / Trade-offs

- Redis 长时间不可用时，副本收敛延迟变为 watcher 周期，不再由 dispatcher 立即重试。缓解方式是保留周期补偿和 readiness lag。
- 大量连续 user-role revision 可能超过单次补偿批次。缓解方式是超过批次时全量失效缓存，牺牲局部精确性换取正确性。
- 删除 outbox 是 schema breaking change。缓解方式是提供向前 migration，并在发布顺序中先 migration 再滚动副本。

回滚需恢复 outbox schema、写入和 dispatcher；不提供向下兼容的双写期。

## Migration Plan

1. 更新规格、Ent schema 和应用代码，生成 Ent 与 Atlas migration。
2. 删除配置、观测和测试中的 dispatcher 依赖。
3. 发布时先执行 migration，再滚动 user-service。旧副本在 migration 后不得再访问 outbox 表，因此不能与新 schema 混跑。

## Open Questions

无。
