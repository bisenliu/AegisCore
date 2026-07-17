## Context

`user-service/internal/features/user/infrastructure/postgres` 当前通过 `UserStoreParams` 暴露 `fx.In` 和 `name:"primary_db"`，使 PostgreSQL adapter 的公开 constructor API 依赖 Fx/Dig metadata。该耦合让基础设施包无法作为普通 Go 组件直接构造，也削弱了 application port 由消费侧拥有、infrastructure adapter 只实现最小接口的边界表达。

本 change 只收敛 user feature 的装配边界：业务调用链、HTTP API、Ent schema、migration、OpenAPI、授权和数据库连接生命周期保持不变。服务仍使用 Fx 作为 runtime composition，但 Fx metadata 只允许停留在 user feature 的 composition 文件或服务级 provider 层。

## Goals / Non-Goals

**Goals:**

- 将 user PostgreSQL store constructor 改为 framework-neutral 的显式 `*ent.Client` 参数。
- 移除 user infrastructure 生产包对 `go.uber.org/fx`、`fx.In` 和 `name:"primary_db"` 的依赖。
- 更新 user feature 的 Fx module，让 composition 层通过 `fx.Annotate` 适配 `primary_db` 命名资源并继续提供 `UserProfileStore` port。
- 更新相关测试，使测试直接使用普通 Go 参数构造 store。
- 扩展架构检查，禁止 user feature 的 domain、application、infrastructure 和 transport 生产包导入 Fx/Dig 或携带 `fx.In`、`fx.Out` metadata。

**Non-Goals:**

- 不删除 `user-service/internal/features/user/fx.go` 或改变服务级 Fx runtime。
- 不改变用户资料 HTTP API、DTO、响应 envelope、错误语义、业务校验或 RBAC 授权行为。
- 不改变 Ent schema、Atlas migration、PostgreSQL/Ent 创建、Ping 或关闭所有权。
- 不推广到 auth、role、permission feature；后续 feature 可单独提 change。

## Decisions

1. PostgreSQL adapter constructor 使用显式 `*ent.Client`。

   原因：`*ent.Client` 是 adapter 实际运行所需的唯一基础设施依赖，直接参数最小且可测试。替代方案是保留 `UserStoreParams` 但移除 `fx.In`，该方案仍会保留只为 DI 形状存在的参数结构，不如直接参数清晰。

2. Fx 命名资源适配停留在 `user-service/internal/features/user/fx.go`。

   原因：`primary_db` 是服务 composition 的命名资源语义，不属于 PostgreSQL adapter 的稳定业务 API。Fx module 可以通过 `fx.Annotate(userpostgres.NewUserStore, fx.ParamTags(`name:"primary_db"`), fx.As(...))` 保持现有生产装配。替代方案是在 providers 层新增 user 专用 wrapper，但会把 feature port 绑定移动到服务 provider，降低 feature module 的局部性。

3. 不保留旧 constructor 或兼容 wrapper。

   原因：本 change 明确为不兼容收敛，旧 API 没有外部模块或持久化数据兼容需求；保留 wrapper 会继续允许 Fx metadata 形状进入 adapter 测试和调用点。替代方案是保留 deprecated wrapper，但会让架构检查和迁移边界变得模糊。

4. 架构检查以 user feature 生产包为本次强约束范围。

   原因：验收目标限定 user feature 的 domain、application、infrastructure 和 transport 生产包；仓库其他 feature 仍存在 Fx metadata，直接全局禁止会扩大 change 范围。检查需要排除 `fx.go`、`fx_test.go`、`*_test.go` 和生成辅助文件，避免拦截允许的 composition 或测试代码。

## Risks / Trade-offs

- [Risk] Fx module 注解缺少 `fx.ParamTags` 会导致生产 graph 无法解析 `primary_db` 命名 `*ent.Client`。→ Mitigation：更新或补充 Fx graph 验证测试，并运行 user feature 包测试。
- [Risk] 架构检查 pattern 过宽可能误报测试、生成代码或 composition 文件。→ Mitigation：在 `architecture-lint-test.sh` 增加 fixture 覆盖违规和排除路径。
- [Risk] 只约束 user feature 会留下其他 feature 的同类耦合。→ Mitigation：本 change 明确限定范围，避免跨 feature 行为变更；后续可按 feature 分批收敛。
- [Risk] 删除 `UserStoreParams` 会破坏旧测试或内部调用点。→ Mitigation：同步更新所有 user feature 调用点，不提供兼容 wrapper。

## Migration Plan

1. 修改 `userpostgres.NewUserStore` 签名为 `func NewUserStore(client *ent.Client) *UserStore`，删除 `UserStoreParams` 和 Fx import。
2. 在 user Fx module 中对 `NewUserStore` 添加 `fx.ParamTags(`name:"primary_db"`)` 并继续 `fx.As(new(userapplication.UserProfileStore))`。
3. 更新 user PostgreSQL adapter 测试和其他直接调用点为 `NewUserStore(client)`。
4. 扩展 `user-service/scripts/architecture-lint.sh` 和对应 fixture 测试，覆盖 user feature 生产包 Fx/Dig 禁止规则。
5. 运行验收命令确认 user feature 测试、OpenSpec validate 和架构检查通过。

回滚方式：如果 graph 或测试失败，可在同一 change 中回退 constructor 签名与 Fx module 注解修改；由于没有 schema、API 或部署资源变更，不需要数据迁移回滚。

## Open Questions

- 无。
