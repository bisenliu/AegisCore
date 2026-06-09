## Why

当前 `user-services/internal/user` 与 `user-services/internal/auth` 已按能力聚合，但 controller、service、domain model、ports、DTO 和 store adapter 仍混放在能力根目录。随着用户资料、认证会话和运行时装配继续增长，需要把业务能力代码进一步拆成稳定的 feature 内部分层，降低包边界歧义和后续新增用例的维护成本。

## What Changes

- **BREAKING（内部 Go import 路径）**：将用户服务业务能力代码统一迁移到 `user-services/internal/features/<feature>/` 下。
- 将认证能力调整为 `features/auth/api`、`features/auth/app`、`features/auth/domain`、`features/auth/store/redis`，分别承载 HTTP DTO、use case/service/commands/ports、领域模型/错误/规则和 Redis adapter。
- 将用户资料能力调整为 `features/user/api`、`features/user/app`、`features/user/domain`、`features/user/store/postgres`，分别承载 HTTP DTO、use case/service/commands/ports、领域模型/错误/规则和 PostgreSQL adapter。
- 保留 `user-services/internal/bootstrap`、`user-services/internal/router` 和 `user-services/internal/validators` 作为服务级进程启动/Fx 装配、HTTP 路由挂载和全局纯函数校验边界。
- 更新 Fx wiring、路由注册、controller/service/store 测试、Swagger 注解、文档和 capability map 中的包路径引用，确保新目录结构成为后续开发的事实来源。
- 保持现有 HTTP API 路径、请求/响应 JSON、响应信封、错误码、配置 key、Redis key、Ent schema 和 Atlas migration 历史不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-domain-boundary`: 用户服务业务能力目录边界从 `internal/user`、`internal/auth` 调整为 `internal/features/user`、`internal/features/auth`，并明确 feature 内 `api`、`app`、`domain`、`store` 分层职责。

## Impact

- 代码影响：`user-services/internal/user`、`user-services/internal/auth` 下的源码和测试会迁移到 `user-services/internal/features/user`、`user-services/internal/features/auth`；`bootstrap`、`router`、`validators` 和 Swagger 注解中的 import 会同步更新。
- 文档影响：`AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/opsx/CAPABILITY_MAP.md` 和相关 OpenSpec 规格中描述目录边界的内容需要同步。
- API 影响：不改变 `/healthz`、Swagger、`/api/v1/auth/*` 或 `/api/v1/users*` 的路径、认证边界、响应信封、错误码或公开字段。
- 数据影响：不修改 Ent schema、生成代码、Atlas migration、PostgreSQL 表结构或 Redis key 格式。
- 依赖影响：不新增第三方依赖，不拆分 Go module，不修改 `go.work` 或 Go toolchain 基线。
