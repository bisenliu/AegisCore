## Context

用户资料创建和查询会从 PostgreSQL adapter 或 application use case 返回 `identity.ErrUserAlreadyExists`、`identity.ErrUserNotFound`。当前用户 HTTP controller 需要先调用 `toUserHTTPError` 将这两个 sentinel error 转换为 `common/contract/errors` 应用错误，再交给 `response.Fail`。这让用户 HTTP 边界拥有一份用户错误到 HTTP 响应的重复映射，而共享 response helper 已经能够通过 `contracterrors.FromError` 直接识别应用错误并渲染 HTTP status、code 和 message。

本变更跨 `user-service/internal/shared/identity`、`user-service/internal/features/user` 和 OpenSpec 文档，但不改变用户 API 路由、请求 DTO、成功响应 data、数据库 schema、OpenAPI 注解、部署资产或观测资产。

## Goals / Non-Goals

**Goals:**

- 将用户不存在、用户已存在定义为可由共享 response helper 直接渲染的应用错误。
- 保留 `errors.Is(err, identity.ErrUserNotFound)` 与 `errors.Is(err, identity.ErrUserAlreadyExists)` 语义，供业务分支和测试继续表达身份错误判断。
- 删除用户 HTTP transport 中仅用于用户错误翻译的 mapper 逻辑，controller 的业务调用失败统一使用 `response.Fail(c, err)`。
- 更新 `user-identity-management` 和 `shared-platform-primitives` delta，固化用户身份错误和 shared kernel 的稳定契约。

**Non-Goals:**

- 不迁移 auth、role、permission 的领域错误。
- 不新增跨 feature 的用户错误全局映射表。
- 不修改用户 API 的业务能力、请求 DTO、成功响应 data 结构、路由、OpenAPI 注解或数据库 schema。
- 不保留 `toUserHTTPError` 或等价兼容函数。

## Decisions

1. 在 `internal/shared/identity` 直接定义应用错误。
   - 选择：`ErrUserNotFound` 使用 `KindNotFound`、`CodeNotFound` 和 identity 包内私有 `Reason`，`ErrUserAlreadyExists` 使用 `KindConflict`、`CodeConflict` 和 identity 包内私有 `Reason`。
   - 理由：错误的业务归属仍在服务内 shared identity，HTTP 渲染由已有共享契约完成，避免在 feature transport 层重复维护相同映射。
   - 备选：继续保留 sentinel error 并在 HTTP controller 映射。该方案保留了重复边界逻辑，不满足本次收敛目标。

2. 依赖 `contracterrors.Error.Is` 保持错误匹配语义。
   - 选择：用户身份错误导出为稳定的应用错误变量，业务代码继续用 `errors.Is` 判断相同错误变量或被包装后的错误。
   - 理由：现有 `contracterrors.Error` 已按 `Kind` 与 `Reason` 支持 `errors.Is` 语义匹配，`FromError` 也会保留已包装的应用错误。
   - 备选：新增业务 sentinel 作为 `Cause` 再包装成应用错误。该方案会增加额外错误变量和迁移复杂度，当前没有必要。

3. controller 只调用 `response.Fail(c, err)`。
   - 选择：`CreateUser`、`GetByUserID`、`ListUsers` 对业务 service 返回错误不再调用用户专用 mapper。
   - 理由：输入准备阶段已经返回应用错误，业务阶段错误也应该通过统一入口归一化，未知错误仍由 `FromError` 渲染为内部错误。
   - 备选：保留一个更薄的 `toUserHTTPError` 包装 `contracterrors.FromError`。这仍是等价兼容函数，且不提供额外稳定语义。

4. 不调整生成物和部署工件。
   - 选择：不运行 Ent migration 或 OpenAPI generate，除非实现过程中发现注解变化。
   - 理由：本变更不改变数据库结构、HTTP 路由、请求/响应 schema 或部署配置。

## Risks / Trade-offs

- [Risk] 应用错误变量如果使用默认 `ReasonConflict` 或 `ReasonNotFound`，不同业务冲突可能按同类错误匹配。→ Mitigation：identity 包使用私有 `Reason` 值区分用户不存在和用户已存在，使 `errors.Is` 针对导出错误变量保持明确语义。
- [Risk] 测试可能仍断言旧 mapper 行为。→ Mitigation：同步调整 user HTTP、command/query 和 shared identity 测试，使断言聚焦 `response.Fail` 输出和 `errors.Is`。
- [Risk] 未知错误绕过 mapper 后的渲染路径变化。→ Mitigation：`response.Fail` 已统一调用 `contracterrors.FromError`，未知错误继续渲染为 `500 Internal Server Error`。

## Migration Plan

1. 更新 `identity/errors.go` 的错误定义和必要测试。
2. 更新用户 command/query/transport 中返回或渲染错误的路径，删除用户专用错误 mapper。
3. 调整相关测试，覆盖 `errors.Is`、409 冲突响应和 404 未找到响应。
4. 运行 `go test ./user-service/internal/shared/identity/... ./user-service/internal/features/user/...`。

回滚方式：恢复 `identity` sentinel error 和用户 HTTP mapper，并回退对应测试与 OpenSpec delta。由于没有数据库、配置或部署变更，回滚不涉及数据迁移。

## Open Questions

无。
