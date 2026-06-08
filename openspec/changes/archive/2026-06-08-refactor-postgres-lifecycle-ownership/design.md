## Context

用户服务运行时在 `user-services/internal/bootstrap` 中声明 `cache_redis`、`user_db` 和 `common_db`。其中 `user_db` 与 `common_db` 通过 `common/runtime/datastorefx` 创建为具名 `*sql.DB` PostgreSQL 连接池，并继续由用户服务 bootstrap 基于这些连接池创建具名 Ent clients。

当前 `common/runtime/datastorefx/postgres.go` 的 PostgreSQL helper 同时负责打开连接池、注册启动 ping 和停止 close lifecycle。`user-services/internal/bootstrap/postgres.go` 在一次 provider 中创建两个 pool，并在第二个 pool 创建失败时手动关闭第一个 pool。`user-services/internal/bootstrap/ent.go` 又为基于这些 pool 创建的 Ent clients 注册停止 close lifecycle。组合后，raw pool、Ent driver 和失败回滚的所有权边界不够清晰。

本变更只处理共享基础设施和用户服务 bootstrap 的 PostgreSQL lifecycle 所有权，不触碰 controller/service/repository 业务分层、HTTP 路由、响应契约、配置键、Ent schema 或 migration。

## Goals / Non-Goals

**Goals:**

- 为具名 PostgreSQL `*sql.DB` pool 建立单一 lifecycle owner，覆盖创建、启动 ping、停止 close 和创建失败回滚。
- 保持 `postgres.<name>` 命名实例、Fx named injection 名称、连接池参数和 ping timeout 行为不变。
- 保持用户服务只创建 `user_db` 与 `common_db`，不因配置中存在 `pay_db` 或其他实例而连接未声明数据库。
- 让 Ent client lifecycle 只清理 Ent 自身资源，不重复承担底层共享 `*sql.DB` pool 的 close 所有权。
- 补充测试覆盖组合 runtime 下的启动 ping、停止释放、Ent close 语义和第二个 pool 创建失败回滚。

**Non-Goals:**

- 不新增 HTTP API、错误码、响应信封字段或路由。
- 不修改 Redis provider 或 Redis lifecycle。
- 不修改 Ent schema、生成代码或 Atlas migration。
- 不引入新的外部依赖。
- 不把 repository、service 或 controller 改为直接打开数据库连接。

## Decisions

1. PostgreSQL raw pool lifecycle 由 datastore/Fx 边界统一拥有。

   `common/runtime/datastorefx` 仍是共享 PostgreSQL runtime helper 的能力边界，负责打开指定 `postgres.<name>` 实例、注册启动 ping 和停止 close。用户服务 bootstrap 可以声明需要的多个 pool，但不应在正常 lifecycle 中与 Ent client 共同拥有 raw pool close。

   备选方案是让 Ent client 关闭底层 pool，PostgreSQL helper 只负责打开和 ping。该方案会削弱共享 helper 的完整 lifecycle 契约，也会让未使用 Ent 的服务需要重新实现 pool close，因此不采用。

2. 用户服务的多 pool 创建需要集中失败回滚。

   `ProvidePostgresPools` 同时创建 `user_db` 与 `common_db`，应通过统一 helper 或清晰的本地流程处理“后一个创建失败时关闭已创建 pool”的情况。回滚必须只处理尚未交给正常 Fx stop lifecycle 管理完成的资源，并保留失败实例名称上下文。

   备选方案是保持调用方手动 `Close`，但该方式会继续把失败回滚散落在服务 bootstrap 层，难以证明与已注册 lifecycle hook 不冲突，因此需要收敛所有权表达。

3. Ent client 不再重复拥有共享 raw pool close。

   Ent clients 仍在 `user-services/internal/bootstrap` 创建并通过具名 `user_db`、`common_db` 注入 repository。实现需要避免 Ent stop hook 再次关闭由 datastore lifecycle 拥有的 `*sql.DB` pool；如果仍需 Ent cleanup，应使用不会关闭外部注入 pool 的 driver/cleanup 方式，或移除会导致底层 pool close 的 Ent close hook。

   备选方案是保留 Ent close 并依赖 `database/sql.DB.Close` 幂等性。该方案无法清晰表达资源所有权，也会让测试只能验证“重复 close 没有爆炸”而非验证“只有一个 owner”，因此不采用。

4. HTTP runtime contract 保持不变。

   `http-service-runtime` 继续通过 CLI/Fx app 启动用户服务，并继续依赖 `shared-infrastructure` 提供 `cache_redis`、`user_db` 和 `common_db`。本变更不改变启动命令、路由注册、中间件顺序、HTTP graceful shutdown 或外部 API，因此只修改 `shared-infrastructure` 的 spec delta。

## Risks / Trade-offs

- [Risk] Ent client close 语义与底层 `*sql.DB` ownership 绑定较深，调整不当可能造成 Ent 内部资源未释放或 raw pool 提前关闭。→ Mitigation：通过组合 lifecycle 测试验证 Ent provider 与 PostgreSQL provider 一起停止时，每个 raw pool 只由预期 owner 关闭，并保留 Ent client 可注入使用。
- [Risk] 第二个 pool 创建失败时，已注册 lifecycle hook 与手动回滚可能出现重复关闭。→ Mitigation：实现集中创建/回滚路径，并补充失败注入测试验证 `user_db` 被释放且错误保留 `common_db` 上下文。
- [Risk] 收敛所有权可能改变现有单元测试对 Ent close 或 PostgreSQL close 的断言。→ Mitigation：更新测试断言到新的规格语义，保留启动 ping、具名注入、未声明 pool 不创建等稳定契约。
- [Risk] Fx hook 停止顺序依赖 provider 注册顺序，未来重排 provider 可能重新暴露歧义。→ Mitigation：测试覆盖组合模块行为，并在实现中避免依赖重复 close 或特定 stop 顺序才能正确释放 raw pool。
