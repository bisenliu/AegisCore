## Context

`rbac-access-control` 当前通过 `user-service/internal/features/permission/infrastructure/casbin` 构建内存 Casbin enforcer，并通过 `common/security/casbin` 暴露无业务语义的三元组授权 wrapper。当前直接依赖为 `github.com/casbin/casbin/v2 v2.135.0`，代码中存在以下直接使用点：

- `user-service/go.mod`：声明 `github.com/casbin/casbin/v2`。
- `user-service/internal/features/permission/infrastructure/casbin/enforcer.go`：导入 `github.com/casbin/casbin/v2`，保存 `*casbin.Enforcer`，调用 `NewEnforcer`、`AddPolicy` 和 `Enforce`。
- `user-service/internal/features/permission/infrastructure/casbin/model.go`：导入 `github.com/casbin/casbin/v2/model`。
- `user-service/internal/features/permission/infrastructure/casbin/model_test.go`：直接构造 enforcer 验证 wildcard model。
- `common/security/casbin`：不直接 import Casbin 模块，只依赖最小 `Enforcer` 接口 `Enforce(args ...interface{}) (bool, error)`。

Casbin v3 使用新的 Go module path `github.com/casbin/casbin/v3`。截至本 proposal 创建时，`go list -m -versions github.com/casbin/casbin/v3` 显示最新稳定版本为 `v3.10.0`，后续 `v3.11.0-snapshot.*` 为快照版本；本次升级默认锁定最新稳定版本，避免把 snapshot 带入生产依赖。

## Goals / Non-Goals

**Goals:**

- 将 user-service 的直接 Casbin 依赖升级到最新稳定 v3，并移除 `/v2` 直接引用。
- 保持现有 RBAC 授权语义不变：未加载 policy、缺少 user role resolver、无角色、策略拒绝、Casbin 执行错误均不得默认放行。
- 明确 v2 到 v3 的不兼容点和适配方案，覆盖模块路径、model 子包、concrete enforcer 类型、策略写入和授权调用。
- 更新测试覆盖，验证 wildcard policy、role subject、route template、HTTP method、common wrapper、permission authorization service 和 HTTP 授权中间件行为不退化。
- 评估 Casbin v3 新能力是否能安全用于诊断或优化，并明确不纳入本次线上授权路径的内容。

**Non-Goals:**

- 不改变 HTTP API、OpenAPI、Ent schema、Atlas migration、RBAC baseline 或 Redis policy sync 协议。
- 不引入 Casbin adapter 持久化 `casbin_rules` 表；持久化权威来源仍为角色、权限和绑定业务表。
- 不把 `common/security/casbin` 扩展为 user-service 专用 policy loader、subject schema 或 route diff 组件。
- 不在本次实现中默认接入 `Explain()` 的 LLM 决策解释能力，不把授权决策依赖外部 LLM 服务。
- 不采用 `v3.11.0-snapshot.*` 快照版本，除非后续明确要求使用快照。

## Decisions

### 1. 依赖锁定到最新稳定 v3

选择：使用 `go get github.com/casbin/casbin/v3@v3.10.0` 更新 `user-service/go.mod` 和 `user-service/go.sum`。

理由：`v3.10.0` 是当前可见的最新稳定 v3；`v3.11.0-snapshot.*` 带有 snapshot 语义，不适合作为默认生产依赖。这样能满足“升级到最新 v3 主版本”的目标，同时避免非稳定快照引入额外发布风险。

备选方案：使用 `@latest`。该方案可能解析到 snapshot 标签，依赖可重复性和发布稳定性较差，因此不采用。另一个备选是保留 v2 并等待 v3 更成熟，但无法提前暴露兼容性问题，不符合本次目标。

### 2. 只替换直接 Casbin import path，不改变业务分层

选择：将所有 `github.com/casbin/casbin/v2` 和 `github.com/casbin/casbin/v2/model` 替换为 `/v3`，保持现有 package alias `casbinlib`、`newModel()`、`Engine.buildEnforcer()` 和 `common/security/casbin.Enforcer` 的职责不变。

理由：当前代码对 Casbin 的直接调用集中在 permission infrastructure 内，common wrapper 已用最小接口隔离 concrete dependency。最小替换能降低主版本升级风险，不向 common 或 application 层泄漏 Casbin v3 类型。

备选方案：把 Casbin v3 concrete type 包装成新的 adapter 接口并改造更多调用链。当前调用面有限，新增 adapter 会增加不必要抽象，因此不采用。

### 3. 保持 `NewEnforcer`、`AddPolicy`、`Enforce` 调用语义并补充兼容检查

选择：继续使用内存 model 创建 enforcer，逐条 `AddPolicy(roleSubject, pathTemplate, httpMethod)`，授权时对用户启用角色逐一执行 `Enforce(role:<role_uuid>, routeTemplate, method)`。

理由：当前 v3 的基础 API 仍覆盖这些调用形态；直接保持既有调用可以确保 policy 权威来源、wildcard 超级管理员、Gin route template 和 HTTP method 语义不变。

需要检查的旧用法和适配方案：

- 旧用法：`github.com/casbin/casbin/v2`。方案：替换为 `github.com/casbin/casbin/v3`。
- 旧用法：`github.com/casbin/casbin/v2/model`。方案：替换为 `github.com/casbin/casbin/v3/model`。
- 旧用法：`*casbinlib.Enforcer` concrete type。方案：保留 concrete type 仅在 permission infrastructure 内部使用，不向 application/common 扩散。
- 旧用法：`NewEnforcer(model)`。方案：升级后编译验证签名，失败时按 v3 构造器签名调整，但不得改为从外部 adapter 或文件加载 policy。
- 旧用法：`AddPolicy` 返回 `(bool, error)`。方案：继续检查 error；返回 false 代表重复 policy 时现有数据唯一性应避免重复，若 v3 行为更严格，应通过测试确认重复不会造成授权失败。
- 旧用法：`Enforce(args ...interface{})`。方案：继续通过 common wrapper 调用，保留 context 取消前置检查和错误包装。

备选方案：改用 Casbin batch 或 role manager 特性重写授权热路径。当前系统的用户角色解析、本地缓存和每角色授权循环已有明确规格，本次依赖升级不重写授权模型。

### 4. v3 新能力仅作为诊断候选，不进入授权决策路径

选择：本次不默认接入 `Explain()` 或 detector 相关能力。可在实现报告中列出后续替换建议：在离线 route diff、授权诊断或管理后台中评估 `Explain()`，但不得影响 request-time allow/deny。

理由：`Explain()` 涉及授权决策解释和可能的 LLM API 集成，不适合作为安全热路径依赖；detector 类能力需要先确认与现有 model、matcher、通配策略和路由模板的实际收益。

备选方案：立即在拒绝访问时调用 `Explain()` 输出原因。该方案可能引入外部依赖、性能开销和敏感 policy 泄露风险，因此不采用。

## Risks / Trade-offs

- [Risk] `@latest` 解析到 snapshot 版本导致不可预期行为 → Mitigation：明确锁定 `v3.10.0`，并在实现时记录 `go list -m` 结果。
- [Risk] Casbin v3 内部 matcher、wildcard 或 policy duplicate 行为变化导致授权结果差异 → Mitigation：运行并补充 `model_test.go`、`enforcer_test.go`、`common/security/casbin` 测试，覆盖允许、拒绝、超级管理员通配和未加载 policy fail-closed。
- [Risk] concrete v3 类型泄漏到 common 或 application 层，扩大升级影响面 → Mitigation：只在 permission infrastructure import v3，common 继续使用最小接口。
- [Risk] 间接依赖变化影响构建或 go.sum → Mitigation：使用 `go mod tidy` 限定在 `user-service` module 内执行，并检查 diff 只包含预期依赖更新。
- [Risk] 使用 v3 新能力过早改变授权路径 → Mitigation：本次只列为后续建议，生产授权仍只依赖 `Enforce` allow/deny。
- [Risk] 升级后遗漏测试中的 `/v2` import → Mitigation：使用 `rg "github.com/casbin/casbin/v2|casbin/v2"` 全仓检查，必须无命中。

## Migration Plan

1. 在 `user-service` module 内执行 `go get github.com/casbin/casbin/v3@v3.10.0`，随后 `go mod tidy`。
2. 替换 Casbin import path：`enforcer.go`、`model.go`、`model_test.go` 从 `/v2` 切换为 `/v3`。
3. 编译并处理 v3 API 差异；优先保持当前 `NewEnforcer`、`AddPolicy`、`Enforce` 调用方式。
4. 运行针对性测试：`go test ./common/security/casbin ./user-service/internal/features/permission/infrastructure/casbin ./user-service/internal/features/permission/application/authorization ./user-service/internal/features/permission/transport/http`。
5. 运行架构和全量验证：`make user-service-architecture-lint`，合并前运行 `make lint` 和 `make verify`。
6. 回滚策略：若 v3 行为或依赖问题无法在本次窗口内解决，恢复 `user-service/go.mod`、`user-service/go.sum` 和 import path 到 v2，并保留兼容性发现记录供后续拆分处理。

## Open Questions

- 是否需要在后续单独 change 中为 RBAC route diff 或管理端加入基于 Casbin v3 `Explain()` 的离线诊断能力。
- 是否需要建立依赖策略，明确生产依赖不得默认采用 `snapshot`、`alpha`、`beta` 或 `rc` 版本。
