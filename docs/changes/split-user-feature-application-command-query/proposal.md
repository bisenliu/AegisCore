# Split user feature application command query

## What

将 user feature 的 application 层拆成更清晰的 command、query 和 validators 子包，为后续用户资料能力扩展预留稳定结构。

目标结构：

```text
user-service/internal/features/user/
  application/
    command/
      create_user.go
    query/
      get_user.go
      list_users.go
    validators/
      user_validator.go
    ports.go
  domain/
  infrastructure/postgres/
  transport/http/
  fx.go
```

本变更迁移和拆分：

- 创建 `user-service/internal/features/user/application/command/`，承载创建用户用例、创建用户 command、创建用户 service。
- 创建 `user-service/internal/features/user/application/query/`，承载按 ID 查询用户、列表查询用户的 query 类型和 query service。
- 创建 `user-service/internal/features/user/application/validators/`，承载 application 层输入规范和业务前置校验。
- 将当前 `application/service.go` 中的创建用户、查询用户、列表用户逻辑按读写拆分。
- 保持 `application/ports.go` 作为 user feature application 层消费侧 ports 根定义，除非实现时发现拆分 ports 更能降低耦合且符合架构规则。
- 更新 HTTP controller 和 mapper，使 controller 继续把 HTTP DTO 映射为 application command/query 后调用用例。

## Why

当前 user application 层只有一个 service，同时承载创建、按 ID 查询和列表查询三个用例。随着用户资料能力继续增长，单一 service 会逐步混合读写依赖、校验规则、日志语义和测试夹具。

将写用例放入 `command`，读用例放入 `query`，并把 application 层校验放入 `validators`，可以带来几个收益：

- 读写职责更清晰，后续新增用户更新、冻结、搜索等能力时不会继续扩张单一 service。
- Controller 到 application 的调用边界更明确，HTTP DTO 仍留在 `transport/http`。
- Store ports 仍由消费侧 application 层拥有，infrastructure adapter 不需要拥有或发明接口。
- 测试可以按用例聚焦，创建用户的密码 hash、UUID、冲突处理不会和列表查询分页测试混在同一个 service fixture 中。

本变更只调整 user feature 内部 application 层组织，不改变外部 HTTP API、响应 envelope、错误码、数据库模型或认证/团队/角色能力。

## Scope

包括：

- 新增 `application/command`、`application/query`、`application/validators` 目录。
- 将 `CreateUserCommand` 和创建用户业务编排迁入 command 层。
- 将 `ListUsersQuery`、按 user_id 查询和用户列表查询业务编排迁入 query 层。
- 在 validators 层放置 application 层输入校验或规范化辅助，避免 controller 和 service 之间出现重复业务前置判断。
- 保持 HTTP request/response DTO、请求绑定、边界解析和简单字段清洗在 `transport/http`。
- 更新 `application/ports.go`、command/query result 类型、Fx module、controller、mapper、controller test 和 application tests 的引用。
- 更新 user feature 相关文档，说明 application 层允许按 `command/`、`query/`、`validators/` 继续细分。
- 运行 `gofmt` 格式化受影响 Go 文件。

不包括：

- 不改变 `GET /api/v1/users/:id`、`POST /api/v1/users`、`GET /api/v1/users` 的 HTTP API。
- 不改变 request/response JSON 字段、状态码、错误码或 response envelope。
- 不引入团队、角色、权限、组织或新的用户域能力。
- 不改变 Ent schema、Ent generated code、Atlas migration 或数据库结构。
- 不改变 PostgreSQL predicate 语义、分页算法、软删除过滤或排序规则。
- 不把 user feature 专属逻辑移动到 `common`、`internal/shared`、`internal/providers` 或 `integration`。
- 不重新新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `user-service/internal/features/user/application/command/` 存在，并承载创建用户 command/use case。
- `user-service/internal/features/user/application/query/` 存在，并承载查询用户 query/use case。
- `user-service/internal/features/user/application/validators/` 存在，并承载 application 层用户输入校验辅助。
- 当前单体 `application/service.go` 不再同时实现创建用户、按 ID 查询和列表用户三个用例。
- Controller 仍负责 HTTP 绑定、HTTP DTO 清洗和边界解析，并调用 application command/query 用例。
- Application command/query 不导入 Gin、HTTP binder、HTTP response、Ent、Redis 或 SQL。
- Infrastructure PostgreSQL adapter 仍只实现 application 层拥有的 ports。
- `GET /api/v1/users/:id`、`POST /api/v1/users`、`GET /api/v1/users` 行为和响应契约保持不变。
- User feature application 单元测试通过。
- User feature controller 测试通过。
- 在 `user-service/` 下运行相关测试通过，至少覆盖 `./internal/features/user/...`。
