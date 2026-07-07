## Context

角色目录、用户角色绑定和角色权限绑定当前会从 domain、application 或 PostgreSQL adapter 返回 `roledomain.ErrRoleAlreadyExists`、`ErrRoleNotFound`、`ErrRoleInvalid`、`ErrSystemRoleProtected`、`ErrRoleInactive`、`ErrUserRoleAlreadyExists`、`ErrUserRoleNotFound`、`ErrRolePermissionAlreadyExists` 和 `ErrRolePermissionNotFound`。这些错误仍是普通 sentinel error，role HTTP controller 需要先通过 `toRoleHTTPError` 转换成 `common/contract/errors` 应用错误，再交给 `response.Fail`。

role HTTP mapper 还重复翻译 `identity.ErrUserNotFound` 和 `permissiondomain.ErrPermissionNotFound`。在用户身份错误和权限目录错误迁移为应用错误后，这些跨 feature 错误应由其归属边界直接携带公开响应契约，role 只负责透传。共享 response helper 已经通过 `contracterrors.FromError` 识别应用错误，且 `contracterrors.Error` 支持按 `Kind` 和 `Reason` 进行 `errors.Is` 匹配。

本变更只收敛角色与 RBAC 绑定错误表达和 role HTTP 边界错误出口，不改变角色、用户角色、角色权限 API 路由、请求 DTO、成功响应 data、RBAC seed 数据模型、Casbin policy sync、数据库 schema、OpenAPI 注解、部署资产或观测资产。

## Goals / Non-Goals

**Goals:**

- 将角色目录、用户角色绑定和角色权限绑定的稳定业务错误定义为可由共享 response helper 直接渲染的应用错误。
- 为每类角色与绑定错误固定独立 `Reason`、`Kind`、`Code` 和中文公开消息。
- 保留 `errors.Is(err, roledomain.ErrRoleNotFound)` 等业务判断语义，供 command/query、seed、infrastructure adapter 和测试继续使用。
- 删除 role HTTP transport 中仅用于角色错误翻译的 mapper 逻辑，controller 的业务调用失败统一使用 `response.Fail(c, err)`。
- 让 role feature 透传 `identity` 与 `permission` 应用错误，不在 role transport 复制跨 feature 映射。
- 更新 `rbac-access-control` 和 `shared-platform-primitives` delta，固化角色和 RBAC 绑定错误迁移后的稳定契约。

**Non-Goals:**

- 不迁移 auth 错误。
- 不改变角色、用户角色、角色权限 API 的业务能力、请求 DTO、成功响应 data 结构、路由、OpenAPI 注解或数据库 schema。
- 不改变 RBAC seed 数据模型、Casbin policy sync、用户角色缓存失效、有效权限聚合或授权判断业务逻辑。
- 不新增跨 feature 的全局错误映射注册表。
- 不保留 `toRoleHTTPError` 或等价兼容函数。
- 不把 permission 或 identity 的错误映射复制到 role 模块。

## Decisions

1. 在 `role/domain` 直接定义角色与绑定应用错误。
   - 选择：角色和绑定错误导出为携带共享错误契约的应用错误变量。`ErrRoleNotFound`、`ErrUserRoleNotFound`、`ErrRolePermissionNotFound` 使用 `KindNotFound` 和 `CodeNotFound`；`ErrRoleAlreadyExists`、`ErrSystemRoleProtected`、`ErrRoleInactive`、`ErrUserRoleAlreadyExists`、`ErrRolePermissionAlreadyExists` 使用 `KindConflict` 和 `CodeConflict`；`ErrRoleInvalid` 使用 `KindValidation` 和 `CodeValidationFailed`。每个错误使用独立 `Reason` 和当前中文公开消息。
   - 理由：错误的业务归属仍在 role domain，HTTP 渲染由已有共享契约完成，避免 transport 层维护重复映射。
   - 备选：继续保留 sentinel error 并在 HTTP controller 映射。该方案保留重复边界逻辑，不满足本次收敛目标。

2. 依赖 `contracterrors.Error.Is` 保持错误匹配语义。
   - 选择：role domain 错误导出为稳定应用错误变量，业务代码继续用 `errors.Is` 判断直接返回或被包装后的错误。
   - 理由：独立 `Reason` 可以避免不同角色错误在同一 `Kind` 下互相误匹配，也无需为同一语义维护 sentinel 与应用错误两套变量。
   - 备选：保留旧 sentinel 作为 cause 并额外包装应用错误。该方案增加双错误变量和迁移复杂度，当前没有必要。

3. role controller 只调用 `response.Fail(c, err)`。
   - 选择：角色目录、用户角色绑定和角色权限绑定 controller 对 command/query 返回错误不再调用角色专用 mapper；输入准备阶段返回的共享应用错误继续直接传给 `response.Fail`。
   - 理由：业务阶段错误迁移后已经携带渲染契约；未知错误仍由 `FromError` 渲染为内部错误。
   - 备选：保留一个薄的 `toRoleHTTPError` 包装 `contracterrors.FromError`。这仍是等价兼容函数，且不提供额外稳定语义。

4. role 透传 identity 与 permission 应用错误。
   - 选择：用户角色绑定中来自 `identity.ErrUserNotFound` 的错误、角色权限绑定中来自 `permissiondomain.ErrPermissionNotFound` 的错误继续由其归属包定义，role application 和 infrastructure 可以返回或包装这些错误，role HTTP controller 只用 `response.Fail` 渲染。
   - 理由：identity 和 permission 是错误语义的拥有者；role transport 复制映射会制造跨 feature 重复契约。
   - 备选：在 role domain 定义用户不存在或权限不存在的代理错误。该方案会模糊错误归属，并使跨 feature 语义重复。

5. 不调整生成物和部署工件。
   - 选择：不运行 Ent migration 或 OpenAPI generate，除非实现过程中发现注解变化。
   - 理由：本变更不改变数据库结构、HTTP 路由、请求/响应 schema 或部署配置。

## Risks / Trade-offs

- [Risk] 角色错误如果复用通用 `ReasonConflict` 或 `ReasonNotFound`，不同业务错误可能按同类错误误匹配。-> Mitigation：role domain 使用独立 `Reason` 值区分每类角色和绑定错误。
- [Risk] role application 或 infrastructure 测试可能仍假设普通 sentinel error 文本。-> Mitigation：同步覆盖直接返回和包装后的 `errors.Is`，避免依赖旧 `Error()` 英文文本。
- [Risk] HTTP 测试可能仍间接覆盖 mapper 行为。-> Mitigation：删除角色专用 mapper 后，测试直接断言 `response.Fail` 输出的 status、code 和中文公开 message。
- [Risk] role 透传 identity 或 permission 应用错误时，如果这些错误尚未迁移完成，HTTP 响应可能退化为内部错误。-> Mitigation：本 change 依赖既有 identity 与 permission 应用错误迁移结果；实现时运行 role 包测试覆盖用户不存在和权限不存在透传响应。
- [Risk] 未知错误绕过 mapper 后的渲染路径变化。-> Mitigation：`response.Fail` 已统一调用 `contracterrors.FromError`，未知错误继续渲染为 `500 Internal Server Error`。

## Migration Plan

1. 更新 `role/domain/errors.go` 的错误定义和必要测试。
2. 更新 role command、query、seed、infrastructure 或 transport 中返回或渲染错误的路径，删除角色专用错误 mapper。
3. 调整相关测试，覆盖 `errors.Is`、409 冲突响应、404 未找到响应、400 validation 响应，以及 identity/permission 应用错误透传。
4. 运行 `go test ./user-service/internal/features/role/...`。
5. 运行 `make user-service-architecture-lint` 验证 OpenSpec 和架构边界。

回滚方式：恢复 role domain sentinel error 和 role HTTP mapper，并回退对应测试与 OpenSpec delta。由于没有数据库、配置或部署变更，回滚不涉及数据迁移。

## Open Questions

无。
