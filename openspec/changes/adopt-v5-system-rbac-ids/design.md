## Context

AegisCore 当前已经把 RBAC 权限目录、系统角色、默认绑定和超级管理员 bootstrap 放在 user-service 边界内维护，并要求 `internal/shared/rbacbaseline` 作为系统基线来源。现有规格仍要求 bootstrap 用户使用固定全零序列 UUID，且权限 ID 的生成来源和固化规则没有形成统一契约。

本 change 面向 AegisCore 被作为基础框架复制、重命名和复用的场景。系统内置 ID 一旦进入数据库、授权策略和审计链路，就属于数据契约；它们不能因项目展示名、服务名、镜像名或 Go module path 调整而漂移。普通运行时业务实体仍应继续使用 UUID v7，以保留当前创建顺序友好的运行时 ID 策略。

受影响路径主要是 `user-service/internal/shared/rbacbaseline/`、role/permission seed、bootstrap super admin application、相关测试、OpenSpec 和文档。该 change 不改变 HTTP API，不改变 Ent schema，不新增 Atlas migration，不调整部署清单或观测资产的运行时命名。

## Goals / Non-Goals

**Goals:**

- 为系统内置 RBAC/bootstrap ID 建立稳定、可审计的 UUID v5 固化常量规则。
- 统一系统 ID 的代码归属：`SystemIDNamespace`、系统角色 ID、bootstrap 用户 ID 和 permission ID 都位于 `rbacbaseline`。
- 保持 `common/runtime/id.NewUUID()` 的职责不变，仅用于运行时新建普通业务实体。
- 用测试校验固化 ID 与 UUID v5 semantic name 的一致性，避免后续维护时误改或重复。
- 明确新项目初始化与已有项目重命名的差异，避免重命名脚本破坏既有 RBAC 数据契约。

**Non-Goals:**

- 不提供旧数据库中既有系统 ID 的在线或离线迁移。
- 不新增数据库表、字段、索引或 Atlas migration。
- 不改变 HTTP API、OpenAPI 请求响应结构、Casbin 授权模型或 RBAC policy sync 机制。
- 不把 user-service 的 RBAC semantic key schema 下沉到 `common`。
- 不引入运行时动态 UUID v5 生成路径；UUID v5 计算只允许在测试或初始化脚本中使用。

## Decisions

### Decision: 系统内置 ID 使用 UUID v5 生成后固化

系统内置角色、权限和 bootstrap 用户 ID 使用 `UUIDv5(SystemIDNamespace, semantic name)` 生成一次后固化为字符串常量。常量注释必须写明 semantic name，例如 `role:super-admin`、`user:bootstrap-super-admin`、`permission:user:create`。

备选方案：继续使用人工全零序列 UUID。该方案虽然直观，但不具备 namespace 和 semantic name 的可验证来源，扩展 permission catalog 时容易产生人工碰撞或审计困难。

备选方案：运行时调用 `uuid.NewSHA1` 动态计算。该方案 deterministic，但会把数据契约变成运行时逻辑，降低 grep、审计和变更评审的透明度。

### Decision: `rbacbaseline` 统一拥有系统 ID

`user-service/internal/shared/rbacbaseline/ids.go` 承载 `SystemIDNamespace`、`SuperAdminRoleID`、`BootstrapSuperAdminUserID` 和全部 permission ID 常量。`DefaultRoles()`、`DefaultPermissions()`、`DefaultRolePermissions()` 必须引用这些常量，bootstrap application 也必须消费 `rbacbaseline.BootstrapSuperAdminUserID`。

备选方案：在各 feature 内分别定义常量。该方案会分散系统基线所有权，增加 role、permission、bootstrap 之间的隐式耦合，并使项目初始化脚本难以集中更新。

### Decision: semantic name 使用业务授权语义

UUID v5 name 必须使用稳定业务语义：`role:<role-key>`、`user:<system-user-key>`、`permission:<resource>:<action>` 或等价资源组动作。不得使用项目名、服务名、HTTP path、中文文案或 Go symbol。

备选方案：使用 HTTP method + path。该方案会在路由重构时导致 ID 漂移，而权限 ID 应绑定授权语义，不应绑定当前 transport 实现细节。

### Decision: 新项目初始化和已有项目重命名分离

新项目初始化脚本可以生成新的 `SystemIDNamespace` 并据此写入固化 ID 常量，但不连接数据库。已有项目重命名脚本不得默认重算 `SystemIDNamespace` 或系统 ID，不得修改已有数据库 RBAC 数据。

备选方案：项目重命名时统一重算系统 ID。该方案会破坏已经写入数据库的角色、权限、用户角色绑定和审计记录，除非配套完整数据迁移；本 change 明确不提供该迁移。

### Decision: 用测试守护 ID 契约

新增 `ids_test.go` 校验 namespace 可解析、所有固化 ID 可解析且版本为 v5、常量等于 `uuid.NewSHA1(namespace, []byte(semanticName))` 的结果、ID 无重复，并校验默认权限与默认绑定只引用已登记常量。

备选方案：仅靠代码评审检查。该方案无法防止后续增删 permission 时误用 UUID v7、内联字符串或重复 ID。

## Risks / Trade-offs

- [Risk] 全新数据库 seed 后系统 ID 将从旧全零序列值变为 UUID v5 常量，已有依赖旧值的外部脚本会失效。→ Mitigation：规格声明只支持全新数据库路径，不提供旧库原地升级；文档和任务中要求更新引用。
- [Risk] 初始化脚本误用于已有项目会造成代码常量与数据库既有数据不一致。→ Mitigation：脚本不连接数据库，重命名脚本默认不得调用初始化逻辑，文档明确仅基础框架创建新项目时运行。
- [Risk] permission semantic name 选取不稳定会在未来重构时诱导 ID 漂移。→ Mitigation：semantic name 禁止使用 path、文案、项目名和 Go symbol，并通过 `ids_test.go` 集中列出映射。
- [Risk] 固化常量需要同时维护常量和测试 case。→ Mitigation：将 semantic name 映射集中在同一测试表，新增 permission 时测试失败会提示补齐。

## Migration Plan

1. 在 `rbacbaseline/ids.go` 新增 namespace 和全部系统 ID 常量。
2. 将 role、permission、role-permission baseline 和 bootstrap application 改为引用统一常量。
3. 增加系统 ID 一致性测试和 baseline 引用完整性测试。
4. 更新相关文档和 OpenSpec delta。
5. 运行 `make user-service-architecture-lint`、相关 Go 测试，并在实现完成后按仓库规则运行 `make lint` 和 `make verify`。

回滚方式：在尚未对数据库执行 seed/bootstrap 前，可通过代码回滚恢复旧常量和引用。若已在全新数据库执行 seed/bootstrap，回滚必须重新初始化该数据库或由 DBA 按受控流程处理 RBAC 数据；本 change 不提供自动回滚脚本。

## Open Questions

- 无。
