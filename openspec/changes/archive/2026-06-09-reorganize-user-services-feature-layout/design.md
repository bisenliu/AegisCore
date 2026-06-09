## Context

用户服务当前已经从横向 `controller/service/repository/dto/domain` 风格收敛到 `internal/user` 与 `internal/auth` 两个能力目录，但能力根目录仍同时承载 controller、service、commands、ports、领域模型、错误、DTO 和 store adapter。认证能力还通过用户 PostgreSQL store 同时满足用户资料持久化端口、认证凭据端口和 token version 端口，边界虽然可用，但对后续维护者不够直观。

本变更只调整 `user-services/internal` 的服务内组织方式，不改变 `common` 模块、HTTP 外部契约、配置、Redis/PostgreSQL 命名实例、Ent schema 或 Atlas migration。目标目录为：

```text
user-services/internal/
  bootstrap/
  router/
  validators/
  features/
    auth/
      api/
      app/
      domain/
      store/
        postgres/
        redis/
    user/
      api/
      app/
      domain/
      store/
        postgres/
```

## Goals / Non-Goals

**Goals:**

- 将用户资料和认证会话代码迁移到 `internal/features/user` 与 `internal/features/auth`。
- 在每个 feature 内明确 `api`、`app`、`domain`、`store` 分层职责。
- 保持 `bootstrap` 作为进程启动、Fx/DI 和基础设施装配边界，保持 `router` 作为 HTTP 路由挂载边界，保持 `validators` 作为全局纯函数校验边界。
- 迁移所有 import、Fx provider、Swagger 注解、测试和文档路径引用，使新目录结构可编译、可测试、可归档。
- 保持现有 HTTP API、响应信封、错误码、认证边界、数据库 schema、Redis key 和配置 key 不变。

**Non-Goals:**

- 不新增用户、认证、授权或管理 API。
- 不改变登录、刷新、强制改密、登出、用户创建、用户查询或用户列表的业务语义。
- 不修改 Ent schema，不运行 Ent codegen，不生成 Atlas migration。
- 不把用户服务特定规则上移到 `common`。
- 不引入 `internal/shared` 或新的横向 `internal/controller`、`internal/service`、`internal/repository`、`internal/api`、`internal/domain` 包。

## Decisions

1. 使用 `internal/features/<feature>` 作为业务能力根目录。

   理由：`features` 明确区分业务能力代码与服务级运行时边界，避免 `internal/user`、`internal/auth` 与 `bootstrap`、`router` 在同一层级混合增长。替代方案是继续在 `internal/user` 与 `internal/auth` 内新增子目录，但能力根目录仍会同时承载多层职责，无法完全表达本次目标树。

2. `app` 包承载 use case/service、commands、ports 和 controller。

   理由：controller 是用例入口的 HTTP adapter，当前 controller 与 service 强耦合于对应 feature 的 commands、ports 和响应映射。把 controller 与 service 放在同一 `app` 包可以保留现有未导出 helper 和测试结构，降低迁移风险。替代方案是新增 `handler` 或 `controller` 子包，但用户目标树没有该层级，且会放大接口导出面。

3. `api` 包只承载 HTTP request/response DTO 和 Swagger 文档模型。

   理由：DTO 是 transport contract，不应包含业务编排、持久化访问或状态规则。Swagger 注解继续通过别名导入 `authapi`、`userapi` 使用这些类型。替代方案是把 DTO 放入 `app`，但会模糊 HTTP transport 与用例编排边界。

4. `domain` 包承载领域实体、枚举、领域错误和领域规则。

   理由：用户状态规则、用户领域实体、认证会话实体、认证领域错误等应独立于 HTTP DTO 和 store 实现。`app` 依赖 `domain`，`store` 将 Ent/Redis 模型映射为 `domain` 类型。替代方案是让 store 或 app 各自维护相似模型，但会增加状态枚举和领域错误漂移风险。

5. 认证 PostgreSQL adapter 与用户 PostgreSQL adapter 分开。

   理由：认证能力需要凭据读取、凭证更新和 token version 操作；用户资料能力需要创建、查询和列表资料。二者可共享具名 `user_db` Ent client，但 adapter 包应按消费能力拆分为 `features/auth/store/postgres` 与 `features/user/store/postgres`，避免一个 user store 同时实现过多跨能力端口。替代方案是继续让用户 store 实现认证端口，但这与目标树中的 `features/auth/store/postgres` 不一致。

6. 服务级 validators 暂时保留在 `internal/validators`。

   理由：用户明确要求保留全局纯函数校验边界。实现时只更新 validators 对 DTO/domain/app 类型的 import，仍禁止其依赖 Gin、Ent、Redis、service、store 或执行业务编排。替代方案是将 user/auth 校验函数分别搬入 feature，但会改变现有全局校验能力边界。

7. 分阶段迁移以编译为准，不做兼容 import shim。

   理由：`internal` 包只被本 module 内消费，保留旧路径 shim 会制造两个事实来源。实现应一次性更新所有 import 与 tests。替代方案是添加旧包转发类型别名，但会延长迁移尾巴，并让后续开发者继续引用旧路径。

## Risks / Trade-offs

- 包路径大规模迁移可能漏改 import 或 Swagger 注解 -> 使用 `rg "internal/(user|auth)"`、`go test ./...` 和 Swagger 生成检查兜底。
- `app` 包同时包含 controller 和 service 可能显得比单独 controller 包更宽 -> 通过文件命名、测试命名和 `api/domain/store` 边界限制职责，避免 controller 承载业务编排。
- 拆出 `features/auth/store/postgres` 可能与用户 PostgreSQL store 存在少量 Ent 映射重复 -> 接受少量重复以换取更清晰的 capability ownership，不引入共享 store helper。
- 现有 OpenSpec 主规格中存在旧路径描述 -> 本 change spec 修改 `user-domain-boundary`，实现阶段同步更新 path-specific specs 和文档，避免归档后规格冲突。
- 目录移动会影响未提交的本地开发分支引用旧路径 -> 这是内部 Go import 的破坏性变更，实施时应在同一 change 内完成所有引用迁移。

## Migration Plan

1. 新建 `user-services/internal/features/{user,auth}` 目录树，按 `api/app/domain/store` 搬迁源码和测试。
2. 将用户资料 controller/service/commands/ports/mapper 放入 `features/user/app`，将用户实体、状态、领域错误放入 `features/user/domain`，将 DTO 放入 `features/user/api`，将 Ent adapter 放入 `features/user/store/postgres`。
3. 将认证 controller/service/commands/credentials/tokens/sessions/ports 放入 `features/auth/app`，将认证会话实体、凭据模型、领域错误、Redis key builder 可按职责放入 `domain` 或 `app`，将 DTO 放入 `features/auth/api`，将 Redis adapter 放入 `features/auth/store/redis`，将凭据与 token version PostgreSQL adapter 放入 `features/auth/store/postgres`。
4. 更新 `bootstrap` 的 Fx provider、route registration、validators、Swagger 注解和 tests 中的 import。
5. 更新 `docs/ARCHITECTURE.md`、`docs/opsx/CAPABILITY_MAP.md`、`AGENTS.md` 和涉及旧路径的主规格。
6. 运行 `gofmt`，在 `user-services/` 执行 `go test ./...`；如 common 未改动，可不运行 common 测试，但最终回归建议仍执行 `common/` 的 `go test ./...`。

回滚策略：由于本变更不修改外部状态、数据库或配置，回滚可通过恢复目录移动和 import 更新完成；无需数据库回滚或 Redis 数据清理。

## Open Questions

- 无需用户进一步决策；实现时若发现某个文件职责无法自然归入 `api/app/domain/store`，优先选择最接近的现有层，并在 tasks 中补充验证点。
