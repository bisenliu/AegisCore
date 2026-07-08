## Why

当前 `user-service` 的 RBAC 授权引擎依赖 `github.com/casbin/casbin/v2 v2.135.0`，而 Casbin 已发布 v3 主版本。升级到最新稳定 v3 可降低长期维护风险，并提前识别授权路径中因主版本变化带来的 API、模块路径、行为和测试兼容性问题。

## What Changes

- 将 `user-service` 中直接依赖的 Casbin 模块从 `github.com/casbin/casbin/v2` 升级为最新稳定 `github.com/casbin/casbin/v3`，当前候选稳定版本为 `v3.10.0`，不采用 `v3.11.0-snapshot.*` 快照版本。
- 全面检查 `common/security/casbin`、`user-service/internal/features/permission/infrastructure/casbin` 及相关测试中所有旧模块路径、API 调用和行为假设。
- 适配 `NewEnforcer`、`AddPolicy`、`Enforce`、model import 等当前使用点，保持现有 fail-closed 授权语义、角色 subject 规则和 route template 匹配行为不变。
- 梳理 v3 可用新能力，评估是否直接用于本次优化；其中 `Explain()` 和 detector 相关能力只作为诊断优化候选，不改变线上授权决策路径。
- 更新依赖锁定和测试验证流程，确保 RBAC seed、policy reload、HTTP 授权中间件、通用 Casbin wrapper 的行为继续满足现有规格。
- **BREAKING**：Go import path 从 `/v2` 切换到 `/v3`，所有直接引用 Casbin 类型或子包的代码必须同步修改；对外 HTTP API、数据库 schema 和 OpenAPI 契约不发生破坏性变化。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`：RBAC 授权实现依赖升级到 Casbin v3，并明确升级后的兼容性检查、授权语义保持和可选诊断优化要求。

## Impact

- 依赖：`user-service/go.mod` 和 `user-service/go.sum` 中的 `github.com/casbin/casbin/v2` 迁移为 `github.com/casbin/casbin/v3`，并检查间接依赖变化。
- 代码：直接影响 `user-service/internal/features/permission/infrastructure/casbin/enforcer.go`、`model.go`、`model_test.go`，以及所有引用 Casbin concrete type、model 子包或测试 helper 的位置。
- 共享契约：`common/security/casbin` 保持最小 `Enforcer` 接口和三元组授权 wrapper，不引入 user-service 业务语义。
- 安全：授权失败、policy 未加载、user role resolver 缺失和 Casbin 执行错误必须继续 fail-closed，不允许升级导致默认放行。
- API 与数据：不变更 HTTP API、OpenAPI、Ent schema、Atlas migration、RBAC 基线数据结构或数据库存量数据。
- 验证：需要运行 Casbin 相关单元测试、permission/role 授权测试、`make user-service-architecture-lint`，合并前运行 `make lint` 和 `make verify`。
