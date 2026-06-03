## Why

当前用户资料查询和创建路径中，repository 层可能直接承担应用错误或 HTTP 响应语义，容易让数据访问层依赖 `common/response` 的应用错误模型。需要把用户不存在、用户已存在这类稳定业务概念沉淀为 domain sentinel error，使 repository 只表达领域事实，service 继续负责映射为统一响应契约。

## What Changes

- 在用户服务 domain 边界定义稳定用户领域错误，例如 `ErrUserNotFound` 与 `ErrUserAlreadyExists`。
- 调整用户 repository，将 Ent not found 与唯一约束错误转换为领域错误，而不是直接返回应用响应错误。
- 调整用户 service，在查询和创建用户时通过 `errors.Is` 识别领域错误，并映射为现有 `response.NotFoundError` 与 `response.ConflictError`。
- 保持 `GET /api/v1/users/:user_id` 与 `POST /api/v1/users` 的 HTTP 状态码、业务错误码、响应信封和对外 message 不变。
- 非目标：不把所有业务错误映射整体下沉到统一 middleware，不改变 controller/service/repository 分层职责，不调整 Swagger 对外错误契约。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `user-profile-query`: 用户不存在错误的内部传播方式调整为 repository 返回用户领域 sentinel error，service 映射为现有 not found 应用错误。
- `user-profile-create`: 用户已存在或数据库唯一约束冲突的内部传播方式调整为 repository 返回用户领域 sentinel error，service 映射为现有 conflict 应用错误。

## Impact

- 影响代码：`user-services/internal/domain`、`user-services/internal/repository/user_repository.go`、`user-services/internal/service/user_service.go` 及对应测试。
- API 兼容性：外部路径、请求参数、响应信封、HTTP 状态码、业务错误码和公开错误消息保持不变。
- 数据模型与迁移：不需要修改 Ent schema、生成代码或 Atlas migration。
- 依赖方向：repository 不再需要依赖应用响应错误模型来表达用户不存在或用户已存在这类领域错误。
