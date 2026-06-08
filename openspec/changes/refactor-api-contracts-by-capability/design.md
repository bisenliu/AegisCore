## Context

当前 `user-services/internal/dto` 包承载用户资料与认证会话的 HTTP API 请求、响应和 Swagger 文档模型。该包内类型主要用于 Gin 请求绑定、共享校验器、service 入参/返回值、Swagger 注释和测试，并未明显混入 repository input、Ent model 或基础设施 helper。

问题在于 `dto` 是技术概念而非业务边界。随着用户资料查询、用户创建、认证会话控制和 Swagger 文档能力持续扩展，继续使用全局 `dto` 包会弱化类型归属，增加后续把不相关模型放入同一目录的风险。本次设计选择按业务 API 能力组织契约模型，而不是按全局 `request`/`response` 技术方向拆分。

## Goals / Non-Goals

**Goals:**

- 将用户资料 HTTP API 契约模型迁移到 `user-services/internal/api/user`。
- 将认证会话 HTTP API 契约模型迁移到 `user-services/internal/api/auth`。
- 在每个业务 API 包内使用 `request.go`、`response.go` 和必要的 `doc.go` 表达请求、响应和 Swagger-only 模型。
- 更新 controller、service、validation、Swagger 注释和测试中的引用，删除不再使用的 `internal/dto` 兜底包。
- 保持 controller/service/repository 分层职责、HTTP API 外部契约、响应信封和错误语义不变。

**Non-Goals:**

- 不新增 API endpoint、认证策略、用户状态或会话行为。
- 不改变 JSON 字段名、校验 tag、Swagger 示例、错误码或响应信封。
- 不拆分 service usecase input/output，也不引入额外 command/query 模型。
- 不修改 `common` 模块、Ent schema、Atlas migration、PostgreSQL/Redis schema 或运行时配置。
- 不手写或重新生成 `user-services/ent/` 生成代码。

## Decisions

1. 按业务 API 能力拆分，而不是保留全局 `dto` 包。

   `dto` 无法表达类型属于用户资料还是认证会话。迁移到 `internal/api/user` 和 `internal/api/auth` 后，调用点可通过 `userapi.CreateUserRequest`、`userapi.UserResponse`、`authapi.LoginRequest`、`authapi.TokenResponse` 直接表达业务归属。

   备选方案是保留 `internal/dto` 并靠文档约束。该方案改动最小，但不能从目录结构上降低兜底风险。

2. 不采用全局 `internal/request` 与 `internal/response` 包。

   全局请求/响应包仍是技术维度分类，并且 `internal/response` 会与 `common/contract/response` 形成命名冲突，增加 import alias 和阅读成本。按业务能力拆分更符合当前 capability map，也更适合后续新增 `session`、`permission` 或 `role` 等业务 API 契约。

3. API 契约包使用 package 名 `userapi` 和 `authapi`。

   目录名保留为 `api/user` 和 `api/auth`，但 Go package 名采用 `userapi`、`authapi`，避免在调用点与 `common/security/auth`、domain user 概念混淆。

4. 暂不拆分 service 专用输入输出模型。

   当前 service 的主要调用方是 HTTP controller，且 API request 已由 controller/validation 完成请求级规范化。立即拆出 service command/query 会引入重复结构和映射代码。本次重构只改善 HTTP API 契约模型的归属，保留现有 service 接口行为。

5. Swagger 注释随类型迁移同步更新。

   controller 注释中的 `dto.*` 引用必须更新为新的 API 契约包类型，确保 `swag` 生成的 schema 与运行时代码一致。Swagger-only 类型如用户列表分页文档模型放在 `internal/api/user/doc.go`，避免混入请求或响应文件。

## Risks / Trade-offs

- [Risk] service 仍依赖 HTTP API contract 类型，未来多入口复用时可能继续存在耦合。→ Mitigation: 本次明确为低风险组织重构；未来若出现 CLI、消息或 gRPC 入口，再通过独立 change 拆分 service command/query。
- [Risk] 大量 import 和 Swagger 注释重命名可能遗漏。→ Mitigation: 使用全仓搜索 `internal/dto`、`dto.`，并运行用户服务测试和 Swagger 相关测试验证。
- [Risk] package 名与目录名不完全一致可能造成初次阅读成本。→ Mitigation: 使用 Go 中常见的显式别名导入形式 `userapi`、`authapi`，调用点语义更清晰。
- [Risk] 重构不改变外部行为，测试失败多半来自引用更新遗漏而非业务逻辑。→ Mitigation: 优先保持结构体字段、tag、方法和注释内容不变，仅迁移包和引用。

## Migration Plan

1. 新建 `user-services/internal/api/user` 和 `user-services/internal/api/auth` 包。
2. 将 `internal/dto/user.go` 中的用户请求、响应和文档类型分别迁移到用户 API 包。
3. 将 `internal/dto/auth.go` 中的认证请求和响应类型迁移到认证 API 包。
4. 更新 controller、service、validation 和测试 imports 与类型引用。
5. 更新 Swagger 注释中的类型引用，确保文档 schema 指向新 API 契约类型。
6. 删除不再使用的 `internal/dto` 包。
7. 在 `user-services` 模块运行 `go test ./...`，必要时运行 Swagger 生成或相关测试验证注释引用。

Rollback 策略：如迁移中发现不可接受的兼容性问题，可恢复 `internal/dto` 包和原 imports；由于不涉及数据、配置、Ent schema 或 migration，回滚不需要数据库或运行时状态迁移。

## Open Questions

- 无。本次 change 仅处理 API 契约模型组织，不改变业务行为或外部 API。
