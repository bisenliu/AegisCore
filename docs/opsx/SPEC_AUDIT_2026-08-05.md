# OpenSpec 主规格审计与精简报告

## 1. 审计范围与口径

本次审计覆盖 `openspec/specs/*/spec.md` 的全部 8 个 capability。数量口径为 `### Requirement:`，代码对齐范围包括 HTTP route graph、feature application/domain/infrastructure、共享 primitive、Ent schema、测试、Makefile、CI 与部署观测资产。

状态含义：

- `[KEEP]`：行为与当前代码一致，保留并统一表述。
- `[MERGE]`：行为有效，但与相邻或跨文档 Requirement 重复或粒度过细，合并到指定 Requirement。
- `[DELETE-UNUSED]`：当前无实现、仅描述一次性迁移或 UI 偏好，且不构成稳定可验证能力。
- `[DELETE-DUPLICATE]`：被权威 capability 覆盖，或与当前保留版本重复。

## 2. 精简统计总览

| 文档路径 | 原 Spec 数量 | 精简后 Spec 数量 | 删减率 | 主要调整原因 |
|---|---:|---:|---:|---|
| `openspec/specs/api-rate-limiting/spec.md` | 5 | 2 | 60.0% | 合并匿名/认证门禁与配置/生命周期，补齐 Purpose 和 fail-open 语义 |
| `openspec/specs/auth-session-management/spec.md` | 9 | 5 | 44.4% | 合并 HTTP 容量、客户端地址与 Cluster 资源，删除重复限流和未实现普通改密 |
| `openspec/specs/delivery-operations/spec.md` | 13 | 8 | 38.5% | 合并 Compose、Redis/可信代理与运行配置，删除无实现初始化规则 |
| `openspec/specs/opsx-foundation/spec.md` | 3 | 2 | 33.3% | 合并治理入口与 change 生命周期 |
| `openspec/specs/rbac-access-control/spec.md` | 23 | 13 | 43.5% | 消除缓存所有权冲突，合并 revision/outbox/watcher/故障验收，删除重复限流 |
| `openspec/specs/runtime-observability/spec.md` | 17 | 8 | 52.9% | 合并 pprof、Redis/HTTP 观测、dispatcher 与 lag，删除 UI 时区和说明性 tracing 条目 |
| `openspec/specs/shared-platform-primitives/spec.md` | 18 | 13 | 27.8% | 合并 Redis、测试、限流、事务契约，修复配置场景错挂，删除 module metadata 条目 |
| `openspec/specs/user-identity-management/spec.md` | 4 | 3 | 25.0% | 将用户写请求体容量合入 HTTP 边界 |
| **合计** | **92** | **54** | **41.3%** | 删除 7 个无用/重复 Requirement，并将 31 个细碎 Requirement 合并到稳定能力 |

Scenario 从 315 个精简为 284 个；规格总行数从 2869 行降为 2519 行。Scenario 的删减小于 Requirement，是因为多数安全、并发与故障验收条件被保留并重新归属。

## 3. 代码对齐证据摘要

| 结论 | 代码证据 |
|---|---|
| 匿名限流在 auth controller 前，认证后限流在 RBAC 前 | `user-service/internal/router/router.go:57-76` |
| 权限 HTTP 能力当前只读 | `user-service/internal/features/permission/transport/http/routes.go:5-9` |
| auth 不存在普通改密路由 | `user-service/internal/features/auth/transport/http/routes.go:5-16` |
| user-role cache 使用 common revision 门禁 | `common/runtime/localcache/cache.go:82-159`、`user-service/internal/features/permission/infrastructure/casbin/user_role_resolver.go:45-57` |
| RBAC revision、业务写和 outbox 同事务 | `user-service/internal/features/role/infrastructure/postgres/tx.go:42-135` |
| outbox dispatcher 与持久 lease 已实现 | `user-service/internal/features/permission/application/dispatcher.go:47-246`、`user-service/internal/features/permission/infrastructure/postgres/outbox_store.go:41-186` |
| watcher 以 PostgreSQL latest revision 为权威 | `user-service/internal/features/permission/infrastructure/redis/watcher.go:168-229` |
| policy reload lag 使用 database latest 与 actual applied 差值 | `user-service/internal/features/permission/infrastructure/redis/watcher.go:418-426` |

## 4. 各文档精简与代码对齐说明

### `openspec/specs/api-rate-limiting/spec.md`

Deleted：无完整 Requirement 删除；删除归档模板的英文 `TBD`。

Merged：

- `[MERGE]` “匿名接口按 IP 限流”与“已认证业务接口按 User ID 限流”合并为“API 限流身份与中间件门禁”。
- `[MERGE]` “本地限流器资源生命周期与清理”“限流配置与默认值”“单实例兜底边界”合并为“本地限流配置、生命周期与部署边界”。

Retained/Refactored：明确 trusted `ClientIP`、JWT/token version/RBAC 顺序、429 envelope、禁用与多副本语义；补充 limiter 内部错误当前采用可观察 fail-open，且不得伪装为调用方超限。

### `openspec/specs/auth-session-management/spec.md`

Deleted：

- `[DELETE-DUPLICATE]` “认证入口限流门禁”：由 `api-rate-limiting` 独占路由顺序与错误语义。
- `[DELETE-UNUSED]` “普通改密/已认证改密”场景：当前没有对应 route、controller 或 use case，仅保留 logout-all 与强制改密撤销语义。

Merged：

- `[MERGE]` “认证入口请求体容量边界”与“登录审计客户端地址”合入“登录、用途隔离令牌与 HTTP 边界”。
- `[MERGE]` “认证 Redis 存储兼容 Redis Cluster”合入“认证架构、配置与 Redis 资源生命周期”。

Retained/Refactored：保留 login、refresh rotation、token version 主事实、一次性强制改密、`UPDATE ... RETURNING` 与撤销不完整错误。

### `openspec/specs/delivery-operations/spec.md`

Deleted：

- `[DELETE-UNUSED]` “项目身份初始化与重命名 ID 边界”：仓库没有初始化/重命名工具；RBAC 固定 ID 的稳定性仍由 RBAC 规格约束。

Merged：

- `[MERGE]` HTTP 请求体上限配置合入“镜像、部署与受控发布”。
- `[MERGE]` 本地时区与 Compose 入口合并为“Compose 本地编排与时区一致性”。
- `[MERGE]` Redis Cluster 配置/发布/回滚与 trusted proxy 交付合并为“Redis Cluster 与可信入口配置交付”。

Retained/Refactored：保留 CLI、质量门禁、Ent/Atlas、不可变工件、Nacos 双目录/双 Namespace 和 Helm 不可变发布。

### `openspec/specs/opsx-foundation/spec.md`

Deleted：无。

Merged：

- `[MERGE]` “OPSX 入口、规范来源与能力治理”与“OPSX change 生命周期与完成门禁”合并为单一治理 Requirement。

Retained/Refactored：保留简体中文 artifact、capability 定位、proposal/apply/archive、暂存后 lint/verify 以及 provider 物理边界。

### `openspec/specs/rbac-access-control/spec.md`

Deleted：

- `[DELETE-DUPLICATE]` 第二份“用户角色缓存失效顺序门禁”：其要求 permission 自建 generation，和当前 common localcache revision 实现相反。
- `[DELETE-DUPLICATE]` “RBAC 业务接口限流门禁”：由 `api-rate-limiting` 统一拥有。

Merged：

- `[MERGE]` user-role cache 配置与强失效顺序合并。
- `[MERGE]` Redis Cluster sync、revision 提交水位、全局刷新、revision gap、写 API 提交语义和事务 outbox 合并为“RBAC policy revision、同步与事务 outbox”。
- `[MERGE]` 并发/故障链路验收合入“RBAC revision 通知、幂等消费与故障验收”。

Retained/Refactored：权限目录明确为代码基线的只读投影；保留系统角色事务保护、Casbin fail-closed、数据库权威 watcher、可靠 outbox、bootstrap 和生产/测试代码边界。提交后本地 reload 失败 MUST 返回已提交 mutation 的成功结果，由 pending outbox 恢复。

### `openspec/specs/runtime-observability/spec.md`

Deleted：

- `[DELETE-UNUSED]` “本地 DB tracing 分层”：仅要求 README 解释 span 层次，没有独立运行时验收价值。
- `[DELETE-UNUSED]` “本地观测时间展示边界”：主要约束浏览器 UI 展示偏好；Compose 进程时区已归 delivery。
- `[DELETE-UNUSED]` 旧 RBAC lag 场景：错误地使用 Redis latest 与 local applied 差值。

Merged：

- `[MERGE]` pprof、Redis command filter、Cluster health/tracing 和 HTTP client IP 合入“运行时故障、诊断与依赖观测边界”。
- `[MERGE]` dispatcher health 与 database-authoritative policy lag 合并为“RBAC 同步可观测性、健康与投影 lag”。
- `[MERGE]` dispatcher 配置与 lifecycle 迁入 RBAC 可靠投递 Requirement。

Retained/Refactored：保留 runtime endpoints、OpenAPI、metrics、watcher staleness、结构化日志、tracing、低基数 route template 与同步健康状态。

### `openspec/specs/shared-platform-primitives/spec.md`

Deleted：

- `[DELETE-UNUSED]` “Go 模块依赖声明”：单个 direct require 属于 `go mod tidy` 与 delivery module metadata 门禁，不是长期平台 capability。

Merged：

- `[MERGE]` Redis mode 配置与 Cluster client lifecycle 合并。
- `[MERGE]` 通用测试基础设施与 Redis Cluster fixture 合并。
- `[MERGE]` 限流错误/HTTP 映射与本地限流 primitive 合并。
- `[MERGE]` transaction helper 与禁止直接 transaction 边界合并。

Retained/Refactored：原先错挂在 Go module Requirement 下的 YAML merge、strict decode、raw digest、effective render 与新 source 场景已迁回“显式配置来源与加载管线”。

### `openspec/specs/user-identity-management/spec.md`

Deleted：无。

Merged：

- `[MERGE]` “用户写接口请求体容量边界”合入“用户资料查询、列表与 HTTP 边界”。

Retained/Refactored：保留原子创建、共享 identity 状态、UUID 查询、keyset 列表、`pg_trgm` 索引、统一错误渲染和 feature 分层。

## 5. 治理来源同步

- `api-rate-limiting` 状态从 `proposed` 修正为 `ready`。
- `AGENTS.md` 与 `openspec/config.yaml` 已将权限能力修正为只读投影，并记录现有 RBAC transaction outbox/dispatcher。
- 能力地图已补充 RBAC policy revision/outbox 同步语义。

## 6. 保留的实现缺口与后续风险

- Helm 不可变镜像 Requirement 仍要求生产使用 digest 或 `sha-<commit>` 等不可变引用，但当前 helper 只拒绝空值和 `latest`。这是实现门禁缺口，不通过弱化规格掩盖，需单独 change 修复。
- outbox delivered event 当前没有 retention/purge 稳定边界。代码和既有产品行为未定义该策略，本次审计不擅自新增；若数据库增长成为运行风险，应创建独立 OpenSpec change。

## 7. 精简后的完整规格索引

以下文件即本次输出的完整、可解析主规格，不在本报告重复复制以避免形成第二份规格来源：

- `openspec/specs/api-rate-limiting/spec.md`
- `openspec/specs/auth-session-management/spec.md`
- `openspec/specs/delivery-operations/spec.md`
- `openspec/specs/opsx-foundation/spec.md`
- `openspec/specs/rbac-access-control/spec.md`
- `openspec/specs/runtime-observability/spec.md`
- `openspec/specs/shared-platform-primitives/spec.md`
- `openspec/specs/user-identity-management/spec.md`

## 8. 验证结果

- `openspec validate --specs`：8/8 通过。
- `make user-service-architecture-lint`：通过。
- `git diff --check`：通过。
