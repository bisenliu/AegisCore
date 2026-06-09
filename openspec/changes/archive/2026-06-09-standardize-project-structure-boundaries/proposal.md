## Why

当前 `user-services/internal` 仍以 controller、service、repository 等技术分层为主，用户、认证等能力边界不够显式；同时 `internal/shared`、`ports.go`、`validators`、`adapter.go`、HTTP request DTO、command/query 和 Ent predicate 的职责边界尚未形成可审查规范，后续重构容易把业务逻辑、基础设施细节或传输层语义扩散到错误位置。

本变更通过标准化项目目录结构和补充 AGENTS.md 架构规则，使用户服务代码按能力聚合，同时把共享边界、端口归属、校验职责、适配器边界、DTO 到 command 映射和 ORM predicate 封装固化为长期可执行规范。

## What Changes

- 将 `user-services/internal` 目标结构调整为按能力组织：`auth/`、`user/` 承载各自 controller、service、ports、model、commands、api DTO 和 store adapter；`bootstrap/`、`router/`、`validators/` 保留为服务运行时和纯校验边界。
- 将用户持久化实现目标边界调整为 `user-services/internal/user/store/postgres/`，并把 Ent predicate 构建封装在该 store 内部。
- 将认证会话 Redis store 目标边界调整为 `user-services/internal/auth/store/redis/`。
- 规范 `api/request.go` 与 `commands.go` 的映射边界：HTTP tag 和绑定语义只留在 request DTO，service 只接收纯应用层 command/query。
- 规范 `ports.go` 为 service 消费侧按用例定义的最小依赖接口，禁止把完整 store CRUD 直接搬入端口。
- 规范 `validators/` 仅承载无状态、无外部依赖的纯业务校验，依赖 DB、Redis 或外部系统的校验必须保留在 service 编排中。
- 规范 `adapter.go` 只能向其他能力暴露极简读取或协作接口，允许字段裁剪和轻量映射，禁止演变为新的业务编排层。
- 将 `internal/shared/` 设为默认不创建的例外目录，只允许无法通过 ports 或依赖注入解决、且多个模块稳定共享的原子级 Value Object 或极少量跨能力错误定义进入。
- 更新 `AGENTS.md`，把以上目录结构和边界规则整理为正式、可执行、可审查的规范语言，并包含允许示例与反例。
- 非目标：不改变 HTTP 路径、请求/响应 JSON 字段、响应信封、错误码、配置 key、Redis key、数据库 schema、Atlas migration 历史或 Go module path。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `common-module-organization`：补充 `common/` 与服务内 `internal/shared/` 的共享能力准入边界，明确 shared 例外目录的审查条件。
- `user-domain-boundary`：补充用户服务按能力聚合后的 service、ports、model、adapter、store 和 Ent predicate 封装边界。
- `user-service-validation-boundary`：补充 `validators/` 纯函数职责、HTTP request DTO 与 command/query 的映射边界，以及 service 不依赖 Gin/HTTP DTO 的约束。

## Impact

- 影响文档：`AGENTS.md`、`docs/opsx/CAPABILITY_MAP.md` 以及相关 OpenSpec change specs。
- 影响代码结构：`user-services/internal/controller`、`user-services/internal/service`、`user-services/internal/repository` 的现有技术分层代码将迁移到 `user-services/internal/user` 与 `user-services/internal/auth` 能力目录；bootstrap 和 router 需要更新 import 与 provider wiring。
- 影响测试：现有 controller、service、repository/store、auth/session 测试需要随包路径迁移并保持行为断言不变。
- 兼容性：本变更属于内部结构和规范重构，不应改变外部 HTTP API、统一响应信封、错误码、配置、Redis key、数据库 schema 或 migration。
