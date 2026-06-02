## Context

项目当前已经在 `common` 中集中维护了标准响应码、分页默认值、validation enum 接口、JWT subject 和部分 HTTP/auth 上下文常量，但仍有几类可复用常量分散在服务侧或 common 子包内部。

主要分散点包括 `user-services/internal/bootstrap` 中的 `user_db`、`common_db`、`cache_redis` 运行时资源名，`common/jwt` 与 `common/contextutil` 之间的 Bearer token type/prefix 关系，以及 `common/middleware/auth.go` 中私有的统一认证失败文案。与此同时，`user-services/ent/` 下存在大量 Ent 生成常量，`user-services/internal/apperror` 中存在用户域业务文案，这两类不适合作为本次公共枚举迁移目标。

## Goals / Non-Goals

**Goals:**

- 建立项目枚举/公共常量归属规则：跨模块、跨服务或跨公共能力复用的契约值进入 `common`；服务私有业务语义保留在服务内。
- 将运行时资源名称 `user_db`、`common_db`、`cache_redis` 提升为 `common` 可复用常量，减少 bootstrap、Ent client、repository 和测试中的重复硬编码。
- 整合 Bearer 认证边界常量，使 token type、Authorization header 和 Bearer prefix 的值由 `common` 统一表达。
- 将通用认证失败公开文案放入 `common` 的响应或认证契约常量，供中间件和测试复用。
- 保持 HTTP 响应、错误码、header、配置 key、Fx 注入 name、数据库 schema 和迁移行为不变。

**Non-Goals:**

- 不引入新的业务错误文案/i18n 框架。
- 不迁移 `user-services/internal/apperror` 中的用户域业务文案。
- 不手写、重命名或迁移 `user-services/ent/` 下的生成常量。
- 不改变 Ent schema、Atlas migration、数据库字段或表名。
- 不通过大规模 Fx wiring 重构替代简单常量迁移。

## Decisions

1. 运行时资源名称放入 `common/infrastructure`。

   理由：这些名称用于 Redis/PostgreSQL/Ent runtime dependency wiring，与 `shared-infrastructure` capability 关联最直接。候选替代方案是放入 `common/config`，但这些名称不是配置结构字段，而是运行时资源 identity；放入 infrastructure 更符合使用语境。

2. 保留 Go struct tag 中的 Fx name 字符串字面量。

   理由：Go struct tag 不能引用常量。为完全移除 tag 字面量而改用 `fx.Annotate` 或自定义 provider helper 会显著扩大重构面，不符合本次最小正确变更目标。实现应在 tag 附近引用同名常量的测试或 provider 调用中保持一致性，并通过测试覆盖防止漂移。

3. Bearer 边界常量优先整合到现有 `common/contextutil` 和 `common/jwt` API，而不是新增顶层 auth constants 包。

   理由：`contextutil` 已经维护 `AuthorizationHeader` 和 `TokenPrefix`，`common/jwt` 已经维护 token subject 和 token type。实现可以通过常量别名或 helper 保持现有导入兼容，避免一次性迁移造成过多调用方变更。

4. 认证失败公开文案放入 `common/response` 的消息常量集合。

   理由：该文案最终体现在统一失败响应信封中，属于对外响应契约的一部分。将其放入 `common/response` 比保留在 middleware 私有常量更利于 controller、中间件和测试共享。

5. 对枚举扫描结果进行显式分类并写入实现清单。

   理由：本变更的风险不在代码复杂度，而在误迁移服务私有语义或生成代码。实现必须产出迁移清单、非迁移清单、引用更新范围和兼容性说明。

## Risks / Trade-offs

- [Risk] 误将用户服务业务文案提升为公共契约，导致 common 依赖业务域语义。→ Mitigation：只迁移认证通用失败文案，保留用户资料、登录凭据、会话缺失等业务文案在 `user-services/internal/apperror`。
- [Risk] Fx tag 字符串无法用常量替换，仍可能存在少量字面量。→ Mitigation：明确 tag 为兼容性边界，使用 common 常量更新 provider 入参、测试数据和非 tag 引用，并增加一致性测试。
- [Risk] Bearer 常量迁移改变导入路径，影响内部调用方。→ Mitigation：优先保留现有 exported names，通过别名或 helper 收敛实现，避免破坏既有 Go API。
- [Risk] 将 Ent 生成常量纳入迁移会破坏生成代码规则。→ Mitigation：实现只审查并列为非迁移项，不编辑 `user-services/ent/`。
- [Risk] 认证失败文案移动后错误码或 HTTP status 被意外改变。→ Mitigation：只替换 message 常量来源，保留 `response.Unauthenticated`、`TokenInvalid`、`TokenExpired` 调用和对应 code/status 测试。

## Migration Plan

1. 扫描 `common/` 和 `user-services/` 中 enum-like `type`、`const` 组、重复字符串和测试断言，形成迁移/非迁移清单。
2. 在 `common/infrastructure` 增加运行时资源名称常量，并更新 bootstrap/provider/test 中可替换的非 tag 引用。
3. 整合 Bearer/Auth 常量，保留现有 `common/jwt` 和 `common/contextutil` 对外名称，更新重复字面量和测试。
4. 在 `common/response` 增加通用认证失败 message 常量，更新 auth middleware 和断言。
5. 运行 `gofmt`，分别在 `common/` 和 `user-services/` 执行 `go test ./...`。
6. 回滚策略：由于不改变外部契约和数据库 schema，如出现问题可回退常量引用改动；无需 Atlas migration 或数据回滚。

## Open Questions

- 暂无阻塞性问题。本次实现应优先采用最小迁移，不引入新公共包；如未来出现更多跨服务业务枚举，再评估独立 `common/enums` 或 i18n/error catalog 能力。
