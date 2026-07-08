## 1. 依赖与旧用法盘点

- [x] 1.1 在 `user-service` module 内确认当前 Casbin 依赖、可用 v3 版本和最新稳定版本，记录不采用 `v3.11.0-snapshot.*` 的原因。
- [x] 1.2 使用 `rg "github.com/casbin/casbin/v2|casbin/v2|casbinlib|NewEnforcer|AddPolicy|Enforce\("` 盘点生产代码和测试中的 Casbin 直接使用点。
- [x] 1.3 列出需要替换、重构或适配的旧用法：模块路径、`model` 子包、`*Enforcer` concrete type、`NewEnforcer`、`AddPolicy`、`Enforce`、测试 helper 和 common wrapper 最小接口。

## 2. Casbin v3 升级实现

- [x] 2.1 在 `user-service` module 内执行 `go get github.com/casbin/casbin/v3@v3.10.0` 并运行 `go mod tidy`，更新 `user-service/go.mod` 和 `user-service/go.sum`。
- [x] 2.2 将 `user-service/internal/features/permission/infrastructure/casbin/enforcer.go` 的 Casbin import 和 `*casbinlib.Enforcer` 使用适配到 v3，保持 `Reload`、`buildEnforcer` 和 fail-closed 语义不变。
- [x] 2.3 将 `user-service/internal/features/permission/infrastructure/casbin/model.go` 和 `model_test.go` 的 `github.com/casbin/casbin/v2/model` 与测试 enforcer import 替换为 v3。
- [x] 2.4 编译处理 v3 API 变更；若 `NewEnforcer`、`AddPolicy` 或 `Enforce` 签名与当前调用不兼容，按 v3 API 最小适配且不得改变 policy 权威来源或授权三元组。
- [x] 2.5 对代码和依赖文件运行 `rg "github.com/casbin/casbin/v2|casbin/v2" common user-service tools --glob '*.go' --glob 'go.mod' --glob 'go.sum'`，确保生产代码、测试代码、`go.mod` 和 `go.sum` 中无旧主版本引用残留。

## 3. 兼容性验证与新特性评估

- [x] 3.1 检查并更新 `common/security/casbin` 测试，确认 allow、deny、底层错误、nil enforcer、context 取消和 `ErrDenied` / `ErrNotConfigured` 语义未因 v3 升级改变。
- [x] 3.2 检查并更新 `user-service/internal/features/permission/infrastructure/casbin` 测试，覆盖普通角色授权、无角色拒绝、policy reload 失败 fail-closed、超级管理员 wildcard policy 和 context 传播。
- [x] 3.3 检查 permission authorization service 与 HTTP 授权中间件测试，确认非法 subject、缺失认证、策略拒绝、授权错误映射和白名单旁路语义不变。
- [x] 3.4 评估 Casbin v3 新特性，包括 `Explain()`、默认 detector、快照版本中的 CSV 字段引用和条件角色管理器修复，记录可直接利用或暂不采用的理由。
- [x] 3.5 输出实现报告，包含升级步骤、影响范围分析、关键代码修改点、旧用法替换清单、新特性优化建议和兼容性注意事项。

## 4. 验证与收尾

- [x] 4.1 运行 `go test ./common/security/casbin ./user-service/internal/features/permission/infrastructure/casbin ./user-service/internal/features/permission/application/authorization ./user-service/internal/features/permission/transport/http` 并修复失败。
- [x] 4.2 运行 `make user-service-architecture-lint`，确认架构边界未因 Casbin v3 concrete type 扩散而破坏。
- [x] 4.3 检查本次 diff，确认不包含 HTTP API、OpenAPI、Ent schema、Atlas migration、部署资产或 RBAC baseline 的非预期变更。
- [x] 4.4 将本次预期代码、依赖和 OpenSpec artifact 变更加到暂存区后运行 `make lint`，失败时修复并重新运行。
- [x] 4.5 在暂存本次预期变更后运行 `make verify`，失败时修复并重新运行，确保最终验证不被未暂存预期 diff 阻塞。

## 5. 实现报告

- 升级步骤：确认当前依赖为 `github.com/casbin/casbin/v2 v2.135.0`；通过 `go list -m -versions github.com/casbin/casbin/v3` 确认最新稳定 v3 为 `v3.10.0`，`v3.11.0-snapshot.*` 属于快照版本；执行 `go get github.com/casbin/casbin/v3@v3.10.0` 并在 import 替换后运行 `go mod tidy`。
- 影响范围：本次代码影响限定为 `user-service/go.mod`、`user-service/go.sum`、`user-service/internal/features/permission/infrastructure/casbin/enforcer.go`、`model.go` 和 `model_test.go`；`common/security/casbin` 继续保持最小 `Enforcer` 接口，不引入 Casbin concrete type。
- 关键代码修改点：`github.com/casbin/casbin/v2` 替换为 `github.com/casbin/casbin/v3`；`github.com/casbin/casbin/v2/model` 替换为 `github.com/casbin/casbin/v3/model`；`NewEnforcer(model)`、`AddPolicy(...)` 和 `Enforce(...)` 在 v3 中保持当前签名，无需兼容 shim。
- 旧用法替换清单：模块路径、model 子包和测试 enforcer import 已全部切换到 v3；`rg "github.com/casbin/casbin/v2|casbin/v2" common user-service tools --glob '*.go' --glob 'go.mod' --glob 'go.sum'` 无命中。
- 新特性优化建议：`Enforcer.Explain` 可生成授权解释，但会调用配置的 OpenAI 兼容 API，不适合进入请求时授权路径；`RunDetections` / detector 能力可后续用于离线模型诊断；snapshot 中的 CSV 字段引用和条件角色管理器修复不应在本次稳定依赖升级中采用。
- 兼容性注意事项：本次不保留 v2 兼容路径、fallback、双轨 adapter 或旧 import；如后续发现 v3 行为不兼容，应直接按 v3 API 和当前 RBAC 规格替换旧用法，而不是恢复或兼容 v2 行为。
