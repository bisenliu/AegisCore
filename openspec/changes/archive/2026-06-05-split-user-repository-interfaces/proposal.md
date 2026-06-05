## Why

当前 `repository.UserRepository` 同时承载用户资料创建、资料查询、认证凭证、token version 和用户列表等多类能力，导致不同消费方被迫依赖并测试自己并不使用的方法集合。这违反接口隔离原则，使 `userService`、认证凭证组件和认证会话组件在仓储抽象层形成不必要耦合，后续任一能力演进都容易跨 capability 影响其他调用方。

## What Changes

- 将根 `repository.UserRepository` 的消费边界拆分为三个高内聚小接口：用户资料仓储、用户凭证仓储、用户 token version 仓储。
- 调整 `userService`、认证凭证组件和认证会话组件的依赖声明，使其只依赖最小必需接口。
- 保持现有 PostgreSQL repository 结构体和方法不拆散，依靠 Go 隐式接口实现同时满足多个小接口。
- 调整 Fx 装配，使同一个底层 PostgreSQL 用户仓储实例可以分别以多个小接口身份注入。
- 收敛单元测试 fake/mock 的方法集合，避免测试替身实现无关仓储方法。
- 不改变现有 HTTP API、响应信封、错误码、配置、Ent schema、Atlas migration 或 Redis 会话行为。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `user-profile-query`: 用户资料查询服务必须只依赖用户资料相关仓储接口，而不是认证相关仓储能力。
- `user-profile-create`: 用户资料创建服务必须只依赖用户资料相关仓储接口，而不是认证凭证或 token version 相关仓储能力。
- `user-session-control`: 认证凭证和会话组件必须分别依赖凭证仓储接口与 token version 仓储接口，而不是完整用户仓储大接口。

## Impact

- 影响代码位置：`user-services/internal/repository/user_repository.go`、`user-services/internal/service/user_service.go`、`user-services/internal/service/auth_service.go`、认证凭证/会话组件、Fx bootstrap 装配和相关单元测试。
- 现有 PostgreSQL 用户仓储实现保留在 `user-services/internal/repository/postgres`，不需要拆散结构体或迁移已有方法。
- 外部 API 兼容：`POST /api/v1/users`、`GET /api/v1/users/:user_id`、用户列表接口、登录、刷新、退出、退出全部设备和修改密码行为保持不变。
- 数据兼容：不涉及 Ent schema 或数据库 migration。
- 测试影响：服务层测试替身按消费端小接口编写，减少无关空实现和跨能力 mock 耦合。
