## Context

当前用户列表接口 `GET /api/v1/users` 的 Controller 在 query binding 和共享校验之后直接调用 `validators.NormalizeListUsers(&req)`。该函数会填充分页默认值、计算 `Offset`/`Limit`，并裁剪 `Nickname`/`Username` 空白字符。这些逻辑虽然位于服务内 `validators` 包，但调用点在 HTTP Controller，使 Controller 承担了影响用例执行和 Repository 查询输入的归一化职责。

现有分层约束要求 Controller 处理 HTTP 解析和响应输出，Service 负责编排，Repository 负责数据库访问。本变更保持 `NormalizeListUsers` 作为用户服务本地校验/归一化边界，但将调用点移入 `UserService.ListUsers`，保证 HTTP 和非 HTTP 调用路径都经过同一归一化流程。

## Goals / Non-Goals

**Goals:**

- 让 `UserController.ListUsers` 只负责 query binding、共享 validation、调用 service 和输出响应。
- 让 `UserService.ListUsers` 在调用 repository 前统一执行列表请求归一化。
- 保持用户列表分页默认值、过滤字段 trim、响应分页 metadata 和错误语义不变。
- 明确未来类似校验的分层边界：结构性请求校验在 Controller/shared validator，业务状态校验和影响用例执行的归一化在 Service。
- 调整单元测试边界，让 Controller 测试不再断言 service 内部归一化细节，Service 测试覆盖归一化结果。

**Non-Goals:**

- 不改变 `GET /api/v1/users` 路由、query 参数、认证要求或响应 envelope。
- 不重构创建用户流程中的 `NormalizeCreateUser` 调用点；如需统一处理创建请求归一化，可后续单独变更。
- 不新增数据库表、Ent schema、Atlas migration、Redis key 或外部依赖。
- 不把用户服务特定规则移动到 `common/validation`。

## Decisions

- 将 `validators.NormalizeListUsers(&req)` 调用移动到 `UserService.ListUsers` 开头。
  - 理由：分页默认值、offset/limit 派生和过滤字段清洗影响 repository 查询输入，属于用例执行前置归一化，Service 是所有调用入口的共同边界。
  - 备选方案：保留 Controller 调用并在 Service 增加兜底归一化。该方案会造成重复归一化职责和测试边界不清，因此不采用。

- 保留 `userapi.ListUsersRequest` 作为 Controller 到 Service 的入参。
  - 理由：当前接口和测试已围绕该 DTO 建立，最小变更可避免引入新的 service input 类型和大范围调用点调整。
  - 备选方案：新增 `service.ListUsersRequest` 或直接让 Controller 构建 `ListUsersInput`。前者增加重映射成本，后者会进一步把 repository 输入知识暴露给 Controller，因此本次不采用。

- 保留 `user-services/internal/validators.NormalizeListUsers` 的副作用式签名。
  - 理由：现有 helper 已有测试覆盖，移动调用点即可修复分层泄露；改成返回值式 API 会扩大非必要变更范围。
  - 备选方案：改为 `NormalizeListUsers(req) userapi.ListUsersRequest`。这可降低副作用风险，但不是本次分层修复的必要条件。

- 更新测试职责边界。
  - Controller 测试只确认 query 参数成功绑定并传入 service，不断言 `Offset`/`Limit` 默认值。
  - Service 测试确认空分页、非法小分页、显式分页、过滤字段空白裁剪后传给 repository 的 `ListUsersInput` 正确。

## Risks / Trade-offs

- [Risk] Controller 测试的既有断言会因归一化不再发生在 Controller 层而失败。→ Mitigation：调整断言为 HTTP binding 边界，并把归一化断言迁移到 Service 单测。
- [Risk] Service 层新增对 `internal/validators` 的 import，可能被误解为 Service 承担 HTTP validation。→ Mitigation：spec 明确该包是服务内校验/归一化边界，不依赖 Gin、DB 或外部资源；共享结构校验仍由 `common/validation` 和 `common/http/ginvalidation` 承担。
- [Risk] 未来复杂业务校验可能再次被放进 request DTO `Validate()` 或 Controller。→ Mitigation：在 `request-validation` delta 中明确需要 repository/cache/业务状态的校验必须由 Service 编排。
- [Risk] 非 HTTP 调用以前可能依赖未归一化输入。→ Mitigation：归一化后行为与 HTTP API 既有行为一致，属于让 Service 契约更稳定的兼容性修复。

## Migration Plan

- 移除 `UserController.ListUsers` 中对 `validators.NormalizeListUsers` 的直接调用。
- 在 `UserService.ListUsers` 开始处调用 `validators.NormalizeListUsers(&req)`，并继续使用归一化后的字段构造 `ListUsersInput` 和分页响应 metadata。
- 调整 Controller 单测，使其只覆盖 HTTP query binding、校验失败和响应输出。
- 调整 Service 单测，使其覆盖默认分页、显式分页、过滤字段 trim 和 repository input。
- 运行 `go test ./...` 验证 `user-services` 模块相关测试。
- 回滚时可恢复 Controller 调用点并移除 Service 调用点；由于没有 schema、配置或外部 API 变化，不需要数据迁移回滚。

## Open Questions

- `CreateUser` 当前仍在 Controller 调用 `validators.NormalizeCreateUser`。本变更将其列为非目标；是否后续用独立 change 统一创建请求归一化边界，需要在后续需求中决定。
