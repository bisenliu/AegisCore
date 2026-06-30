## Context

auth application 当前通过 `UserTokenVersionStore` 管理 PostgreSQL 中的用户 token version，通过 `AuthSessionStore` 同时管理 Redis token version 投影和 refresh session 生命周期。`AuthSessionStore` 包含缓存读取/写入/删除、session 创建/轮换/查询/删除和批量删除，导致 token version validator、session lifecycle 和 Redis adapter 之间的依赖面偏大。

全部会话撤销目前由 application lifecycle 编排：先递增 PostgreSQL token version，再刷新 Redis token version 投影并删除全部 refresh session。现有实现把后半段视为投影失败并记录日志，use case 仍返回成功。该策略可以保证旧 access token 因 PostgreSQL token version 递增而失效，但 Redis cache 和 refresh session 删除失败时的补偿责任没有通过 port 或文档清晰表达。

本次 change 只调整 user-service auth feature 内部契约和测试，不改变 HTTP API、数据库 schema、OpenAPI、Redis key schema、Prometheus 指标名称、label key、部署清单或 Grafana/Prometheus 资产。

## Goals / Non-Goals

**Goals:**

- 将 auth session 相关 port 拆成最小依赖面，使 token version cache、refresh session store 和 token version 持久化仓储可以被不同 application 组件分别依赖。
- 明确全部会话撤销的一致性语义：PostgreSQL token version 递增是旧 token 失效的主事实，Redis token version 投影和 refresh session 批量清理是必须尝试且可观测的投影步骤。
- 让投影失败的处理策略在代码中显式表达，包括返回值、日志或补偿任务边界，避免调用方误以为所有存储动作具备单一事务。
- 更新单元测试覆盖 port 拆分和撤销投影失败语义。

**Non-Goals:**

- 不引入跨 PostgreSQL 与 Redis 的分布式事务。
- 不新增 outbox、eventbus、MQ、后台可靠投递系统或跨服务补偿框架。
- 不改变 JWT claims、token version 数值语义、refresh session Redis key schema 或 session rotation 原子脚本。
- 不改变 HTTP endpoint、响应 envelope、OpenAPI 注解、数据库字段、Ent schema、Atlas migration、Casbin policy 或部署观测资产。
- 不改变 auth metrics port、Prometheus 指标名称、label key 或 label value。
- 不把 auth 业务 port 上移到 `common` 或 `user-service/internal/shared`。

## Decisions

### Decision: 拆分 auth application port

将现有 `AuthSessionStore` 的职责拆成类似 `TokenVersionCache`、`RefreshSessionStore` 和已有 `UserTokenVersionStore` 的最小接口。token version validator 只依赖 token version 持久化仓储与 cache；session lifecycle 只在需要创建、轮换、查询或撤销 refresh session 时依赖 refresh session store；撤销编排组件同时依赖 token version store、token version cache 和 refresh session store。

备选方案：保留 `AuthSessionStore`，只调整注释说明。该方案无法降低测试桩和调用方依赖面，也无法通过类型系统表达 ISP 边界，因此不采用。

### Decision: 保持撤销为应用层编排，不下沉到 Redis adapter

全部会话撤销涉及 PostgreSQL token version、Redis token version 投影、本实例 localcache 失效和 refresh session 删除。业务语义仍应位于 `application/sessions` 或 command 编排中，Redis adapter 只实现 token version cache 和 refresh session store 的存储契约。

备选方案：在 Redis adapter 中提供 `IncrementTokenVersionAndDeleteAllSessions` 一类组合方法。该方案无法包含 PostgreSQL 递增，也会把业务撤销语义塞进 infrastructure，不符合当前 feature 分层，因此不采用。

### Decision: 采用最终一致撤销并显式化投影失败

用户 token version 在 PostgreSQL 中递增成功后，旧 access token 的安全失效事实已经成立；Redis token version 投影刷新和 refresh session 物理删除必须尽力执行并可观测。实现时应让 `RevokeAllUserSessions` 或其内部结果类型明确区分主事实成功与投影失败，调用方可以按现有产品语义返回成功，同时日志和测试必须能证明投影失败没有被静默吞掉。

备选方案：只要 Redis 投影或 session 删除失败就让全部登出返回失败。该方案会让用户在 token version 已递增后看到失败，但旧 token 已经失效，重试可能再次递增版本并扩大副作用；本次不采用。

## Risks / Trade-offs

- [Risk] port 拆分会影响大量测试桩和 Fx 装配 → Mitigation：先定义最小接口，再用编译错误逐步迁移调用点，相关 auth application、Redis adapter 和 Fx 测试必须覆盖。
- [Risk] 最终一致撤销可能被误读为“Redis 删除失败也完全成功” → Mitigation：在 result、日志或补偿任务命名中显式表达 projection failure，并在规格和测试中固定语义。
- [Risk] token version cache 写入失败后存在短暂 Redis stale cache → Mitigation：撤销流程必须先失效本实例 localcache，cache 写入失败时必须尝试删除 Redis 投影；Redis miss 后回源 PostgreSQL，旧版本不得覆盖新版本。

## Migration Plan

1. 在 `auth/application/ports.go` 定义拆分后的 token version cache 与 refresh session store port，保留或调整 `UserTokenVersionStore` 的最小职责。
2. 更新 `application/validators`、`application/sessions` 和 command 依赖，使每个组件只接收自身需要的 port。
3. 更新 Redis session store adapter，使同一 concrete type 可以实现新的 cache/store 小接口，但不承载应用层撤销编排。
4. 调整全部会话撤销结果或错误处理，使主事实成功与投影失败的语义可测试、可观测。
5. 运行 auth 相关测试、Redis session store 测试、`make user-service-architecture-lint`，并确认 OpenAPI、Ent 生成物没有漂移。

回滚方式：该 change 未修改数据库、Redis key 或 HTTP API；实现阶段可通过回退 Go 代码和测试恢复旧 port 与 metrics `string` 签名。若已合并后需要回滚，必须同步回退所有接口实现、Fx provider 和测试桩，避免半迁移状态导致编译失败。

## Open Questions

无。当前方案明确采用最终一致撤销，不引入跨存储事务或新的可靠消息基础设施。
