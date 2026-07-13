## Context

当前 `role` 和 `permission` 公开 HTTP 创建接口允许请求体携带 `system` 字段，HTTP input preparer 将该字段映射到 application command，普通 command 再传递到普通 PostgreSQL store create，最终写入 `roles.is_system` 或 `permissions.is_system`。

系统保护逻辑依赖 `is_system` 判断是否禁止破坏性修改，因此普通 RBAC 管理员可通过公开 API 制造受系统保护的数据。与此同时，后续新增 API 权限会通过代码基线写入并由 RBAC seed 落库，系统角色和系统权限本应只由 seed port 写入。现状的问题是公开 API 能直接制造 `is_system=true`，而不是缺少额外来源字段。

本变更影响 `user-service/internal/features/role`、`user-service/internal/features/permission`、OpenAPI 生成物与 `rbac-access-control` 主规格。`common` 不承载 user-service 业务 DTO 或 seed 语义；本变更不修改 Ent schema、不生成 Atlas migration、不调整部署资产。

## Goals / Non-Goals

**Goals:**

- 公开角色和权限创建 API 不再允许调用方制造系统角色或系统权限。
- 普通 role/permission command、port 和 store create 路径不再携带调用方可控的系统标记。
- 普通角色和权限创建路径固定写入 `is_system=false`。
- 仅 RBAC seed port 能写入 `is_system=true` 的角色和权限。
- OpenAPI、单元测试、store 测试和 OpenSpec delta 与新安全边界保持一致。

**Non-Goals:**

- 不保留旧 API 中 `system` 字段的兼容语义。
- 不新增 provenance、source、created_by 或其他来源字段。
- 不修改角色或权限数据库 schema，不生成 Atlas migration。
- 不新增独立管理系统数据的 HTTP API。
- 不改变 Casbin subject/object/action 模型、policy loader、policy sync、用户角色缓存或超级管理员通配授权语义。
- 不修复历史上已由公开 API 制造的错误系统数据；历史数据清理由运维或后续专门 change 处理。

## Decisions

### 决策一：公开 API 破坏性移除 `system` 创建字段

`CreateRoleRequest` 和 `CreatePermissionRequest` 删除 `System` 字段，`prepareCreateRoleCommand` 与 `prepareCreatePermissionCommand` 不再构造 `IsSystem`。OpenAPI 生成物同步删除请求 schema 中的 `system` 字段。

选择该方案是因为 `system` 是系统保护标记，不是普通 RBAC 管理员可设置的业务属性。保留字段并强制忽略虽然能阻断写入，但会让公开契约继续暗示客户端可设置系统性；改为校验 `system=false` 也会保留旧契约分支。由于用户要求不保留兼容方案，本变更直接移除公开字段。

备选方案：保留 `system` 字段但强制置 false。该方案对客户端兼容更好，但无法从契约上消除误用入口，且与“公开 API 移除 `system`”要求不一致，因此不采用。

### 决策二：普通 command/port/store create 移除调用方可控 `IsSystem`

`CreateRoleCommand`、`CreatePermissionCommand`、`CreateRoleInput` 和 `CreatePermissionInput` 删除 `IsSystem`。普通 store create 显式写 `is_system=false`。

选择该方案是为了让安全边界下沉到 application port 和 infrastructure store，而不只依赖 HTTP DTO。即使未来出现其他 transport 或内部调用普通 command，也无法绕过公开 HTTP 层制造系统数据。

备选方案：仅删除 HTTP request 字段。该方案能修复当前 HTTP 入口，但 application command 仍保留危险参数，未来新增调用方时容易复发，因此不采用。

### 决策三：seed port 固定写系统标记

`SeedRoleInput` 和 `SeedPermissionInput` 不应再让调用方决定 `IsSystem`；`UpsertSystemRole` 和 `UpsertSystemPermission` 根据 seed port 语义固定写 `is_system=true`。`rbacbaseline` 继续作为系统角色、系统权限和默认绑定的代码基线来源。

选择该方案是为了把“可写系统数据”的能力绑定到专门端口，而不是普通输入字段。后续新增 API 权限也通过代码基线进入 `rbacbaseline`，再由 RBAC seed 写入系统权限。

备选方案：继续保留 `Seed*Input.IsSystem` 并由 seed service 填 true。该方案可工作，但没有彻底消除输入层可控系统标记，边界表达不如 seed port 固定写清晰，因此不采用。

### 决策四：不新增 provenance 字段

本变更不新增 provenance 字段。系统性边界由两个稳定路径表达：普通公开 API 固定写 `is_system=false`，RBAC seed port 固定写 `is_system=true`。在后续新增 API 权限均通过代码基线写入的约束下，`is_system` 足以表达当前安全边界。

备选方案：新增 provenance 字段区分 `api` 与 `seed`。该方案能增强审计，但会引入数据库 schema、migration、response 和 OpenAPI 额外变更；在当前已明确新增 API 权限通过代码写入的前提下，不符合最小变更原则，因此不采用。

### 决策五：历史错误数据不在本变更自动清理

本变更只阻断普通 API 后续制造系统数据，不自动把现有非基线 `is_system=true` 数据降级或删除。历史清理涉及生产数据判断，可能影响授权关系和角色权限绑定，应通过运维审计或专门数据修复 change 处理。

备选方案：自动将所有非基线 UUID 的 `is_system=true` 改为 false。该方案有误伤风险，且不应在本次 API 边界修复中混入复杂数据修复，因此不采用。

## Risks / Trade-offs

- 旧客户端继续提交 `system` 字段可能被忽略或拒绝 → 通过 OpenAPI 破坏性更新明确公开契约变更；若当前 JSON binder 不拒绝未知字段，本变更至少保证字段不会产生系统写入能力。
- 不新增 provenance 字段会降低来源审计粒度 → 通过 seed-only 写入路径和测试保证新增数据边界；历史异常数据由后续数据审计或清理处理。
- 已存在的错误系统数据不会自动修复 → 本变更阻断新增风险，避免在安全边界修复中误伤生产数据。
- 如果部分测试依赖当前 `system=true` 创建行为，会被反转 → 测试必须改为断言公开 API 不能创建系统数据、seed port 才能写系统数据。

## Migration Plan

1. 更新普通 role/permission 创建路径，删除公开 `system` 输入并固定普通 create 为 `is_system=false`。
2. 更新 RBAC seed 路径，固定 seed upsert 为 `is_system=true`。
3. 更新 OpenAPI 注解和生成物，使公开创建请求不再包含 `system`。
4. 更新单元测试、store 测试和 seed 测试，覆盖普通 API 固定非系统与 seed 固定系统。
5. 运行 `make user-service-openapi-generate`、相关包测试和 `make user-service-architecture-lint`。

回滚策略：由于本变更不修改数据库 schema，回滚仅涉及代码和 OpenAPI 生成物。但回滚到旧代码会重新允许 `system=true` 创建，因此安全回滚只适用于临时恢复服务，随后必须重新应用修复。

## Open Questions

无。
