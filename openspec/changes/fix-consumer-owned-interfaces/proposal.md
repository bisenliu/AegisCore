## Why

当前用户资料 controller、service、repository 之间的接口声明位置与消费方向不一致，导致高层业务用例依赖低层 repository 包中的端口和输入模型。这个问题会增加替换持久化实现、拆分用例依赖和维护 Fx 组装时的耦合成本，需要在继续扩展用户资料与认证相关能力前修正边界。

## What Changes

- 将用户资料 service 消费的持久化端口移动到 service 消费侧声明，避免 service 层依赖 repository 包结构。
- 将用户资料 service 使用的仓储输入模型移动到 service 消费侧，PostgreSQL 仓储实现适配该端口。
- 清理为错误接口位置产生的 `AsUserProfileRepository` 胶水 provider，并在 bootstrap 组装层完成具体实现到消费方端口的绑定。
- 保持现有 HTTP API、响应信封、错误码、配置、Ent schema 和数据库迁移不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-profile-query`: 查询用户资料的实现边界应由消费方声明端口，避免 service 依赖 repository 层接口。
- `user-profile-create`: 创建用户资料的实现边界应由消费方声明端口，避免 service 依赖 repository 层输入模型和接口。

## Impact

- 影响代码：`user-services/internal/service/user_service.go`、`user-services/internal/repository/user_repository.go`、`user-services/internal/repository/postgres/user_repository.go`、`user-services/internal/bootstrap/app.go` 及相关测试替身。
- API 兼容性：无外部 HTTP 行为变更，`GET /api/v1/users/:id`、`GET /api/v1/users`、`POST /api/v1/users` 的请求、响应和错误语义保持不变。
- 数据兼容性：不修改 Ent schema，不生成 Atlas migration。
- 依赖注入：Fx 绑定从 repository 实现包中的转换函数转移到 bootstrap 组装层或 Fx 注解。
