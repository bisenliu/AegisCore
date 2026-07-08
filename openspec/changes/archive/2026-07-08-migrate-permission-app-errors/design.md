## Context

权限目录创建、查询、更新、启停和系统权限保护路径当前会返回 `permissiondomain.ErrPermissionAlreadyExists`、`ErrPermissionNotFound`、`ErrPermissionInvalid` 或 `ErrSystemPermissionProtected`。这些错误仍是普通 sentinel error，permission HTTP controller 需要先通过 `toPermissionHTTPError` 转换成 `common/contract/errors` 应用错误，再交给 `response.Fail` 渲染。

共享 response helper 已经通过 `contracterrors.FromError` 识别应用错误，且 `contracterrors.Error` 支持按 `Kind` 和 `Reason` 进行 `errors.Is` 匹配。本变更只收敛权限目录错误表达和 HTTP 边界错误出口，不改变权限目录 API 路由、请求 DTO、成功响应 data、Casbin policy sync、route diff、数据库 schema、OpenAPI 注解、部署资产或观测资产。

## Goals / Non-Goals

**Goals:**

- 将权限不存在、权限已存在、权限输入无效和系统权限保护定义为可由共享 response helper 直接渲染的应用错误。
- 为每类权限错误固定独立 `Reason`、`Kind`、`Code` 和中文公开消息。
- 保留 `errors.Is(err, permissiondomain.ErrPermissionNotFound)` 等业务判断语义，供 command/query、infrastructure adapter 和测试继续使用。
- 删除 permission HTTP transport 中仅用于权限错误翻译的 mapper 逻辑，controller 的业务调用失败统一使用 `response.Fail(c, err)`。
- 更新 `rbac-access-control` 和 `shared-platform-primitives` delta，固化权限错误迁移后的稳定契约。

**Non-Goals:**

- 不迁移 role、user、auth 的领域错误。
- 不新增跨模块权限错误映射注册表。
- 不修改权限目录 API 的业务能力、请求 DTO、成功响应 data 结构、路由、OpenAPI 注解或数据库 schema。
- 不改变 Casbin policy sync、用户角色缓存失效、route diff 诊断或授权判断业务逻辑。
- 不保留 `toPermissionHTTPError` 或等价兼容函数。

## Decisions

1. 在 `permission/domain` 直接定义权限应用错误。
   - 选择：`ErrPermissionNotFound` 使用 `KindNotFound`、`CodeNotFound` 和 `permission_not_found`；`ErrPermissionAlreadyExists` 使用 `KindConflict`、`CodeConflict` 和 `permission_already_exists`；`ErrPermissionInvalid` 使用 `KindValidation`、`CodeValidationFailed` 和 `permission_invalid`；`ErrSystemPermissionProtected` 使用 `KindConflict`、`CodeConflict` 和 `system_permission_protected`。公开消息复用当前权限中文文案。
   - 理由：错误的业务归属仍在 permission domain，HTTP 渲染由已有共享契约完成，避免在 transport 层重复维护权限错误映射。
   - 备选：继续保留 sentinel error 并在 HTTP controller 映射。该方案保留重复边界逻辑，不满足本次收敛目标。

2. 依赖 `contracterrors.Error.Is` 保持错误匹配语义。
   - 选择：权限错误导出为稳定应用错误变量，业务代码继续用 `errors.Is` 判断直接返回或被包装后的错误。
   - 理由：`contracterrors.Error` 已按 `Kind` 与 `Reason` 支持语义匹配，独立 `Reason` 可以避免不同权限错误在同一 `Kind` 下互相误匹配。
   - 备选：保留原 sentinel 作为 `Cause` 并额外包装应用错误。该方案会增加双错误变量和迁移复杂度，当前没有必要。

3. controller 只调用 `response.Fail(c, err)`。
   - 选择：权限目录生命周期、用户有效权限查询和 route diff controller 对 command/query 返回错误不再调用权限专用 mapper。
   - 理由：输入准备阶段已有共享应用错误，业务阶段错误迁移后也应通过统一入口归一化；未知错误仍由 `FromError` 渲染为内部错误。
   - 备选：保留一个薄的 `toPermissionHTTPError` 包装 `contracterrors.FromError`。这仍是等价兼容函数，且不提供额外稳定语义。

4. 不调整生成物和部署工件。
   - 选择：不运行 Ent migration 或 OpenAPI generate，除非实现过程中发现注解变化。
   - 理由：本变更不改变数据库结构、HTTP 路由、请求/响应 schema 或部署配置。

## Risks / Trade-offs

- [Risk] 权限错误如果复用通用 `ReasonConflict` 或 `ReasonNotFound`，不同业务错误可能按同类错误误匹配。-> Mitigation：permission domain 使用独立 `Reason` 值区分每类权限错误。
- [Risk] command/query 测试可能仍假设普通 sentinel error 文本。-> Mitigation：同步覆盖直接返回和包装后的 `errors.Is`，避免依赖旧 `Error()` 英文文本。
- [Risk] HTTP 测试可能仍间接覆盖 mapper 行为。-> Mitigation：删除权限专用 mapper 后，测试直接断言 `response.Fail` 输出的 status、code 和中文公开 message。
- [Risk] 未知错误绕过 mapper 后的渲染路径变化。-> Mitigation：`response.Fail` 已统一调用 `contracterrors.FromError`，未知错误继续渲染为 `500 Internal Server Error`。

## Migration Plan

1. 更新 `permission/domain/errors.go` 的错误定义和必要测试。
2. 更新 permission command/query/transport 中返回或渲染错误的路径，删除权限专用错误 mapper。
3. 调整相关测试，覆盖 `errors.Is`、409 冲突响应、404 未找到响应和 400 validation 响应。
4. 运行 `go test ./user-service/internal/features/permission/...`。
5. 运行 `make user-service-architecture-lint` 验证 OpenSpec 和架构边界。

回滚方式：恢复 permission domain sentinel error 和 permission HTTP mapper，并回退对应测试与 OpenSpec delta。由于没有数据库、配置或部署变更，回滚不涉及数据迁移。

## Open Questions

无。
