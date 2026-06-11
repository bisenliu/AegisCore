# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`，确认本变更只调整 user feature application 层组织。
- [x] 查看当前 `user-service/internal/features/user/application/service.go`、`commands.go`、`ports.go`、`result.go`、`transport/http/controller.go`、`transport/http/mapper.go` 和 `fx.go`。
- [x] 记录当前 user feature 测试入口，至少包括 `user-service/internal/features/user/application/service_test.go` 和 `transport/http/controller_test.go`。

## Package Structure

- [x] 创建 `user-service/internal/features/user/application/command/`。
- [x] 创建 `user-service/internal/features/user/application/query/`。
- [x] 创建 `user-service/internal/features/user/application/validators/`。
- [x] 保留 `user-service/internal/features/user/application/ports.go` 作为 application 层根部消费侧 port 定义。
- [x] 取消 `user-service/internal/features/user/application/result.go`，将 result 类型放到对应 command/query 用例文件。

## Command Use Case

- [x] 将 `CreateUserCommand` 从 `application/commands.go` 迁移到 `application/command`。
- [x] 将当前 `service.CreateUser` 逻辑迁移为 command 层创建用户 use case。
- [x] 创建 command service constructor，例如 `NewCreateUserService`。
- [x] 保持默认用户状态逻辑不变。
- [x] 保持密码 hash 使用 `common/security/password.HashContext` 不变。
- [x] 保持 UUID V7 生成逻辑不变。
- [x] 保持 `ErrUserAlreadyExists` 透传语义不变，不在 application 层映射 HTTP 错误。
- [x] 确认 command package 不导入 Gin、HTTP binder、HTTP response、Ent、Redis 或 SQL。

## Query Use Cases

- [x] 将 `ListUsersQuery` 从 `application/commands.go` 迁移到 `application/query`。
- [x] 为按 user_id 查询用户定义 query 层输入或保留明确的 UUID 参数签名。
- [x] 将当前 `service.GetUserByID` 逻辑迁移到 query 层。
- [x] 将当前 `service.ListUsers` 逻辑迁移到 query 层。
- [x] 保持 `ErrUserNotFound` 透传语义不变，不在 application 层映射 HTTP 错误。
- [x] 保持列表分页 `Limit + 1` 的 store 契约、`NextCursor` 生成和 `HasNext` 语义不变。
- [x] 创建 query service constructor，例如 `NewUserQueryService`。
- [x] 确认 query package 不导入 Gin、HTTP binder、HTTP response、Ent、Redis 或 SQL。

## Validators

- [x] 在 `application/validators` 中创建用户 application 层输入校验辅助。
- [x] 只放置 transport-neutral 规则，避免复制 HTTP DTO binding tag 或字段标签逻辑。
- [x] 确认 validators 不导入 Gin、HTTP request/response DTO、HTTP response、Ent、Redis 或 SQL。
- [x] 为 validators 添加聚焦单元测试。

## Root Application Contracts

- [x] 更新 `application/ports.go` 中的输入类型引用，使其能被 command/query 子包使用。
- [x] 如需拆分读写 ports，确保新 ports 仍属于 application 层并由 command/query 消费侧拥有。
- [x] 保持 `UserProfileStore` 或拆分后的 ports 与 PostgreSQL adapter 方法语义一致。
- [x] 更新 result 引用，确保 command/query 用例拥有自己的 transport-neutral result。
- [x] 删除或收敛旧的单体 `application/service.go`，避免根 application service 继续同时实现三个用例。
- [x] 如保留根部 facade 或组合接口，确保不会形成 import cycle。

## HTTP Transport

- [x] 更新 `transport/http/controller.go` 构造函数，使 controller 依赖 command service 和 query service。
- [x] 更新 `CreateUser` handler，构造 command 层 `CreateUserCommand` 后调用 command service。
- [x] 更新 `GetByUserID` handler，构造 query 输入或传递 UUID 后调用 query service。
- [x] 更新 `ListUsers` handler，构造 query 层 `ListUsersQuery` 后调用 query service。
- [x] 保持 HTTP DTO 绑定、Normalize、ParseUserID、ParseListCursor 和错误响应逻辑在 `transport/http`。
- [x] 更新 `transport/http/mapper.go`，移除对 application 根部 result 的依赖。
- [x] 更新 controller tests 的 stub，使其反映 command/query 拆分后的依赖。

## Infrastructure And Fx

- [x] 更新 `infrastructure/postgres/user_store.go` 的 application import 和 interface assertion。
- [x] 更新 `infrastructure/postgres/predicates.go` 如其引用 application input 类型。
- [x] 更新 PostgreSQL adapter tests 的 application import。
- [x] 更新 `user-service/internal/features/user/fx.go`，提供新的 command/query services。
- [x] 如 controller 依赖接口，使用 `fx.As` 标注 command/query provider。
- [x] 确认服务级 `internal/providers` 不承载 feature 业务逻辑。

## Documentation

- [x] 更新 `AGENTS.md` 中 user feature application 分层说明，加入 `command/`、`query/`、`validators/`。
- [x] 更新 `AGENTS.md` Key Entry Points，将 user service 入口指向新的 command/query 文件。
- [x] 更新 `AGENTS.md` Repository Rules，说明 controller 映射到 command/query 并保持 application 依赖规则。
- [x] 更新 `docs/ARCHITECTURE.md` Feature-First Organization 和 HTTP Request Flow。
- [x] 更新 `docs/DEVELOPMENT.md` 中新增 user feature 用例时的目录指引。
- [x] 如 `docs/TESTING.md` 提到旧 application service test 路径，同步更新。
- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。

## Formatting

- [x] 对受影响 Go 文件运行 `gofmt -w`。
- [x] 运行 `go test` 前确认 Go import alias 清晰，例如 `usercommand`、`userquery`、`userapplication`。

## Verification

- [x] 运行 `test -d user-service/internal/features/user/application/command`。
- [x] 运行 `test -d user-service/internal/features/user/application/query`。
- [x] 运行 `test -d user-service/internal/features/user/application/validators`。
- [x] 运行 `cd user-service && go test ./internal/features/user/...`。
- [x] 如果 Fx wiring、providers 或跨 feature imports 受影响，运行 `cd user-service && go test ./...`。
- [x] 运行结构扫描，确认不存在仍同时实现三个用例的根部单体 service：

```bash
rg -n 'type service struct|func \(s \*service\) CreateUser|func \(s \*service\) GetUserByID|func \(s \*service\) ListUsers' user-service/internal/features/user/application
```

- [x] 检查 `git diff -- user-service/internal/features/user AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md`，确认没有 HTTP API、Ent schema、migration 或无关重构变更。

## Review Notes

- [x] 确认 `GET /api/v1/users/:id` 行为和响应契约无变化。
- [x] 确认 `POST /api/v1/users` 行为和响应契约无变化。
- [x] 确认 `GET /api/v1/users` 分页和过滤行为无变化。
- [x] 确认 response envelope、错误码和状态码无变化。
- [x] 确认 HTTP request/response DTO 没有移动到 application 层。
- [x] 确认 application command/query/validators 没有导入 Gin、Ent、Redis、SQL 或 HTTP response。
- [x] 确认 PostgreSQL adapter 只实现 application 层 ports。
- [x] 确认没有新增团队、角色、权限或组织能力。
- [x] 确认没有修改 Ent generated code、Ent schema、Atlas migration 或部署资产。
