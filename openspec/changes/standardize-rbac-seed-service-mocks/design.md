## Context

`user-service/internal/features/role/application/seed` 负责 RBAC 系统角色、系统权限、系统角色权限绑定和超级管理员用户角色绑定的离线 seed 编排。该包通过 role application port 消费 `SeedRoleStore`、`SeedRolePermissionStore`、`SeedUserRoleStore`，并通过 permission application port 消费 `SeedPermissionStore`。

该包已有 `mock_generate.go` 覆盖 role application seed port，并已有 `mock_permission_test.go` 覆盖 permission application 的 `SeedPermissionStore`。测试迁移应使用这些生成物表达依赖调用，避免手写 store double 把调用记录、状态变更、重复 seed 返回和错误注入隐藏在自定义结构体中。

## Goals / Non-Goals

**Goals:**

- 将 RBAC seed service 测试中的外部持久化协作者统一迁移到已有 gomock 生成物。
- 移除 `seedRoleTestStore`、`seedPermissionTestStore`、`seedRolePermissionTestStore`、`seedUserRoleTestStore` 等实现 seed port 的手写 double。
- 用 expectation、`gomock.InOrder`、matcher 或 `DoAndReturn` 明确表达默认 seed、重复 seed、reactivate、sync bindings 和 assign super admin 的调用顺序、调用次数、参数和返回值。
- 用循环和 matcher 覆盖 `rbacbaseline.DefaultRoles()`、`DefaultPermissions()`、`DefaultRolePermissions()` 对应的 seed 调用数量和参数映射。

**Non-Goals:**

- 不修改 RBAC baseline catalog、seed service 生产逻辑、RBAC CLI 或 PostgreSQL seed adapter。
- 不迁移 role command/query、permission feature、HTTP transport 或跨 feature 测试。
- 不新增生产接口、跨 feature seed mock、共享 mock 仓库或中央测试替身包。
- 不改变 HTTP API、OpenAPI、数据库 schema、Atlas migration、Redis policy sync、部署清单、观测资产或安全边界。

## Decisions

### Decision: 以已有生成 mock 作为唯一 seed collaborator 表达

实现时使用 `mock_generate.go` 已生成的 `NewMockSeedRoleStore`、`NewMockSeedRolePermissionStore`、`NewMockSeedUserRoleStore`，以及 `mock_permission_test.go` 已生成的 `NewMockSeedPermissionStore`。这样测试断言与 production port 方法签名保持同源，接口变化会通过编译错误、mockgen drift 或 expectation drift 直接暴露。

备选方案是继续保留手写 seed store double 并补充更多调用记录字段。该方案会维持两套协作者契约，且难以表达禁止调用、精确调用次数、ordered expectation 和 mockgen drift，因此不采用。

### Decision: 用 ordered expectation 表达 seed 编排顺序

默认 seed、重复 seed、reactivate 和 sync bindings 场景应按 service 当前编排顺序声明 expectation：先 upsert 系统角色，再 upsert 系统权限，再 ensure 或 sync 系统角色权限绑定。assign super admin 场景只声明 `SeedUserRoleStore.AssignRole` 调用，并校验内置超级管理员角色 ID。

备选方案是仅断言最终 `SeedResult` 或 `AssignSuperAdminResult`。该方案不能发现 role、permission、binding 顺序漂移，也不能验证重复 seed 和 reactivate 参数是否正确传入端口，因此不采用。

### Decision: 用循环和 matcher 覆盖 baseline catalog 映射

测试应从 `rbacbaseline.DefaultRoles()`、`DefaultPermissions()`、`DefaultRolePermissions()` 构造 expectation，并通过 matcher 校验 `SeedRoleInput`、`SeedPermissionInput` 和角色权限绑定 ID 集合。对 permission upsert 返回值需要保持 `actualPermissions` 映射时，使用 `DoAndReturn` 或精确返回对象维持 service 后续 binding 输入。

备选方案是手写固定数量 expectation 或大量使用 `gomock.Any()`。固定数量容易与 baseline catalog 扩展漂移，`gomock.Any()` 会丢失参数映射断言，因此仅对与当前断言无关的 context 参数放宽。

### Decision: 不为了测试迁移引入生产抽象

本变更只调整 seed 包测试和既有生成 mock 的使用方式。若 `make user-service-generate` 暴露生成物 drift，只更新对应 mock 生成物；不得为了让测试更容易而新增生产接口、adapter 或共享 helper。

备选方案是提取新的测试基础设施或跨 feature seed mock 包。该方案扩大了影响面，并与本次仅标准化 seed service 测试的目标不匹配，因此不采用。

## Risks / Trade-offs

- [Risk] expectation 过细导致测试对无关调用顺序敏感 -> Mitigation：只对 seed service 明确编排顺序使用 `gomock.InOrder`，对 matcher 内集合比较按业务语义处理顺序。
- [Risk] baseline catalog 增减权限后测试维护成本上升 -> Mitigation：基于 `rbacbaseline.Default*` 循环生成 expectation，让数量随 baseline 自然变化，并保留参数 matcher 发现映射错误。
- [Risk] `DefaultRolePermissions()` 内部调用 `DefaultPermissions()` 导致测试对 catalog 调用次数理解错误 -> Mitigation：测试不 mock baseline 函数，只用其返回值构造期望，并断言 store port 调用数量与返回 catalog 长度一致。
- [Risk] 生成 mock 与 `mock_generate.go` 或 `mock_permission_test.go` 不一致 -> Mitigation：执行 `make user-service-generate`，确认 seed 相关 mock 无 drift。

## Migration Plan

1. 梳理 `service_test.go` 中手写 seed store double 的使用点，并按默认 seed、重复 seed、reactivate、sync bindings 和 assign super admin 分组。
2. 使用 `NewMockSeedRoleStore`、`NewMockSeedPermissionStore`、`NewMockSeedRolePermissionStore` 和 `NewMockSeedUserRoleStore` 替换对应测试替身。
3. 将旧 double 内的状态断言迁移为 expectation、matcher、`gomock.InOrder` 或 `DoAndReturn`。
4. 删除不再使用的手写 seed store double 类型及其方法，保留不实现外部 port 的纯构造 helper 和 matcher。
5. 执行 `make user-service-generate`，确认 `mock_generate.go` 和 `mock_permission_test.go` 无 mockgen drift。
6. 执行 `cd user-service && go test ./internal/features/role/application/seed`。
7. 执行 `make user-service-architecture-lint`。

回滚方式是还原本次测试文件和 OpenSpec change artifacts；由于不改生产代码、schema、配置或部署资产，不需要运行时回滚步骤。

## Open Questions

- 无。
