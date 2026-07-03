## Context

permission HTTP transport 由 `PermissionController` 处理权限目录生命周期、用户有效权限查询和 route diff 诊断端点。生产代码通过 `binding.BindOrAbort` 完成 URI、query 和 JSON 绑定，再由 feature-local input preparer 构造 permission application command/query，并通过 mapper 输出 response envelope。现有测试覆盖了部分 input、scanner 和授权中间件，但 controller handler 仍有多个 0% 覆盖路径，缺少直接测试固定 HTTP status、response envelope、错误映射、分页数据和 application port 调用。

本次 change 属于 `rbac-access-control` capability 的测试覆盖补强。它只修改 `user-service/internal/features/permission/transport/http` 测试与本 change artifacts，不调整生产 handler、request/response DTO、OpenAPI 注解、Casbin enforcer、policy sync、Redis watcher、permission PostgreSQL store、数据库 schema 或部署资产。

## Goals / Non-Goals

**Goals:**

- 补齐 permission HTTP controller 级测试，覆盖 `ListPermissions`、`CreatePermission`、`GetPermission`、`UpdatePermission`、`EnablePermission`、`DisablePermission`、`ListUserEffectivePermissions`、`RouteDiff` 和内部 `setPermissionActive` 的主要成功与失败路径。
- 固定请求绑定、UUID/cursor/query 解析、application port 入参、错误映射、分页 envelope、route diff response 和有效权限 response 映射。
- 使用现有 gomock mock 表达 `PermissionCommandService` 与 `PermissionQueryService` 调用，保持 transport 测试位于 application port 边界。
- 遵循 `docs/TESTING.md` 和 `delivery-operations` 主规格，默认使用 `testify/require`，仅在多个互相独立响应字段需要一次性呈现差异且后续检查不依赖前置结果时使用 `assert`。
- 明确不新增旧权限资源路径、旧 action/resource 字段语义、旧错误 envelope、旧 route scanner 输出、旧授权绕过或兼容 helper 断言。

**Non-Goals:**

- 不修改 permission 生产 controller、request/response DTO、mapper、input preparer、routes 或 scanner 行为，除非实现中发现现有行为与已生效规格明显不一致并需要最小修复。
- 不新增 e2e 数据库流程、Ent schema、Atlas migration、OpenAPI 生成物、RBAC seed 数据、Casbin policy 行为、Redis watcher 或 policy sync 行为。
- 不迁移 permission application、infrastructure/postgres、role feature 或 common 测试断言风格；这些范围由其他 change 覆盖。
- 不新增测试专用生产接口、兼容 helper 或手写 fake 来替代已有 gomock collaborator。

## Decisions

1. 以 controller 级测试作为主要覆盖层。

   理由：本次缺口位于 HTTP boundary，controller 级测试能直接验证 Gin 绑定、input preparer、application port 调用、错误映射和 response envelope，同时避免 e2e 数据库、Casbin 或 Redis 依赖带来的慢速与不稳定。

   备选方案：只扩展 `input_test.go` 与 `mapper_test.go`。该方案能覆盖解析和映射函数，但无法验证 handler 是否使用正确 binder、是否调用正确 service、是否返回正确 HTTP status 和 envelope。

2. 复用 existing gomock port mock。

   理由：permission HTTP controller 已依赖 `PermissionCommandService` 和 `PermissionQueryService`，gomock expectation 可以直接约束 command/query 入参和调用次数，不需要引入 infrastructure adapter、store fake 或真实外部依赖。

   备选方案：创建手写 spy/fake。该方案会增加测试专用类型，并可能弱化对 port 调用次数、错误路径和入参结构的约束。

3. 按 endpoint 行为族组织测试。

   理由：权限目录生命周期、有效权限查询和 route diff 分别有不同的 URI、query、body 和 response 形态。按行为族拆分测试可以让 fixture 更小，并让失败定位靠近对应 handler。

   备选方案：创建一个大表驱动覆盖所有 handler。该方案减少辅助函数数量，但不同 endpoint 的 binder、分页、UUID 解析和 response 差异较大，容易降低可读性。

4. 不为旧兼容路径写断言。

   理由：`no-compat` 语义要求测试固定当前 HTTP 契约，而不是继续接受旧权限资源路径、旧 action/resource 字段、旧 envelope、旧错误码或旧 route scanner 输出。若测试发现历史兼容入口，应避免新增对应断言。

   备选方案：同时验证旧字段和新字段。该方案会把过期兼容语义固化进规格，增加后续边界收敛成本。

## Risks / Trade-offs

- [Risk] controller fixture 过度抽象会隐藏 handler 行为。-> Mitigation: 只保留构造 Gin context、解析 envelope、构造领域对象等低层辅助函数，核心 expectation 和断言放在具体测试中。
- [Risk] 只做 controller 单元测试无法覆盖真实认证和 RBAC middleware。-> Mitigation: 本 change 目标是 permission HTTP boundary；认证授权中间件已有独立测试和 e2e 流程覆盖，不在本 change 重复。
- [Risk] gomock 入参匹配过宽导致测试不能发现 input preparer 退化。-> Mitigation: 对 command/query 使用 `gomock.Eq` 或 custom matcher 校验关键 UUID、分页、过滤字段、状态布尔值和 body 归一化结果。
- [Risk] 新增测试可能需要补齐 mock 生成物。-> Mitigation: 使用既有 `mock_generate.go` 和 `go generate` 入口，不手写生成代码；若现有 mock 已覆盖所需接口则不触碰生成物。
- [Risk] 错误映射测试可能与 common response envelope 细节耦合。-> Mitigation: 仅断言 HTTP status、contract error code、success 标志和必要 message，不断言无关序列化细节。

## Migration Plan

1. 扫描 permission HTTP controller、request、response、mapper、input preparer 和现有测试，列出权限目录、有效权限和 route diff endpoint 的覆盖缺口。
2. 如现有 gomock 生成物不足，使用既有生成入口更新 `mock_*_test.go`；否则只新增普通 `_test.go`。
3. 为权限目录生命周期 handler 补齐 controller 测试，再为用户有效权限和 route diff handler 补齐测试。
4. 使用 `gofmt` 格式化修改过的测试文件，并运行 permission HTTP 包覆盖率测试。
5. 运行 permission feature 测试和 OpenSpec 校验；若全仓 `make lint` / `make verify` 因无关 active change 或 runtime 文件阻塞，记录具体原因。

回滚方式：本次不修改生产行为或持久化结构；如新增测试存在不可接受的问题，可回退对应 `_test.go` 和本 change artifacts，不需要数据库、部署或 API 回滚。

## Open Questions

- 无待决问题；实现阶段若发现某个 endpoint 已有同等 controller 覆盖，应保留更清晰的现有覆盖并避免重复测试。
