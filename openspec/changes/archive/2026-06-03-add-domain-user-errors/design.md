## Context

用户服务当前保持 controller/service/repository 分层：controller 处理 HTTP 解析与响应输出，service 负责编排和应用错误映射，repository 负责 Ent 数据访问。用户不存在和用户已存在属于稳定业务概念，但相关数据访问路径仍可能直接返回 `common/response` 应用错误，导致 repository 承载 HTTP/应用响应语义。

本变更覆盖 `user-profile-query` 与 `user-profile-create`。目标是在 `user-services/internal/domain` 中定义用户领域 sentinel error，让 repository 只返回领域错误或包装后的内部错误；service 使用 `errors.Is` 将领域错误转换为现有 `response.NotFoundError` 或 `response.ConflictError`。

## Goals / Non-Goals

**Goals:**

- 在 domain 包中定义 `ErrUserNotFound` 与 `ErrUserAlreadyExists`，作为用户领域稳定错误边界。
- repository 将 Ent not found 与唯一约束错误转换为 domain sentinel error。
- service 继续负责把 domain sentinel error 映射为 `common/response` 应用错误。
- 保持用户查询和创建 API 的 HTTP status、业务 code、响应 envelope 和对外 message 不变。
- 通过单元测试覆盖 repository 错误转换和 service 错误映射。

**Non-Goals:**

- 不引入统一错误 middleware 来替代 service 层映射。
- 不改变 controller 的响应输出职责。
- 不修改 Ent schema、生成代码、数据库迁移或 Swagger 对外错误文档。
- 不重构所有认证、会话或其他用户 repository 错误路径，除非它们直接复用查询/创建路径并需要保持一致。

## Decisions

1. 领域错误放在 `user-services/internal/domain`

   选择在 domain 包定义 `ErrUserNotFound` 和 `ErrUserAlreadyExists`，因为它们表达用户领域事实，而不是 repository 实现细节或 HTTP 响应语义。相比放在 repository 包，domain 包更便于 service、repository 和测试共享，也避免将业务概念绑定到数据访问包。

2. repository 返回 sentinel error，不返回 `response.*Error`

   repository 在 `ent.IsNotFound(err)` 时返回 `domain.ErrUserNotFound`，在 `ent.IsConstraintError(err)` 时返回 `domain.ErrUserAlreadyExists`。其他数据库错误继续用 `fmt.Errorf` 包装上下文，交由 service 的 `response.FromError` 转换为内部错误。这样能保持依赖方向由 service 指向 response，repository 不依赖应用响应模型。

3. service 使用 `errors.Is` 做应用错误映射

   service 在 `GetUserByID`、`CreateUser` 的 repository 返回错误分支中优先识别 domain sentinel error，再返回 `response.NotFoundError(errmsg.MsgUserNotFound)` 或 `response.ConflictError(errmsg.MsgUserAlreadyExists)`。相比直接比较错误值，`errors.Is` 能兼容未来 repository 包装领域错误的场景。

4. 不把业务错误映射下沉到 middleware

   当前项目中 `common/response` 是应用错误模型，controller/service 已显式返回 `response.*Error`。如果改由 middleware 统一映射，需要重新规范 service 是否允许返回纯 domain error、controller 是否统一调用 `response.FromError`、panic/recovery 和普通业务错误如何区分，以及 Swagger 错误码文档如何保持一致。本变更选择更小的分层修正，避免扩大架构调整范围。

## Risks / Trade-offs

- [Risk] 只处理查询和创建相关领域错误，其他用户仓储方法仍可能返回应用错误 → Mitigation: 本变更按 proposal 范围聚焦 `user-profile-query` 与 `user-profile-create`，实现时可识别直接复用 `GetByUserID` 的路径并避免破坏现有认证/会话行为。
- [Risk] service 漏掉 `errors.Is` 映射会把领域错误转换成 500 → Mitigation: 为 service 查询不存在用户、创建唯一冲突路径增加单元测试。
- [Risk] 对外中文 message 发生变化 → Mitigation: service 映射继续使用 `user-services/internal/errmsg` 的既有 message 常量。
- [Risk] repository 测试如果只断言 response 错误会失效 → Mitigation: 更新测试断言 `errors.Is(err, domain.Err*)`，controller/service 测试继续断言外部响应契约。

## Migration Plan

1. 新增 domain sentinel error 定义。
2. 更新用户 repository 查询和创建错误转换。
3. 更新用户 service 错误映射逻辑。
4. 更新或新增 repository/service 单元测试。
5. 在 `user-services/` 运行 `go test ./...` 验证服务模块。

无需数据库 migration、Ent 代码生成或运行时配置变更。回滚时可恢复 repository 直接返回应用错误的旧逻辑，但不应改变 API 对外响应。

## Open Questions

无。
