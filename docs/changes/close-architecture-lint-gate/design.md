# Design

## Overview

本变更关闭 Go lint 与架构依赖检查的执行闭环。当前仓库已经有统一入口、lint 配置和 CI workflow，但仍处在软门禁状态：本地 lint 会失败，CI workflow 用 `continue-on-error: true` 吞掉失败。

设计目标是先把两个 module 的 lint 基线清零，再把 CI lint job 改为硬失败。这样 depguard 与其他 lint 规则都能成为真实门禁，而不是只在日志里提示。

## Current Baseline

本 change 创建时的本地验证结果：

- `make lint-common` 失败，52 个 findings：
  - `errcheck`: 2
  - `gofmt`: 1
  - `goimports`: 30
  - `govet`: 11
  - `revive`: 6
  - `staticcheck`: 2
- `make lint-user-service` 失败，65 个 findings：
  - `depguard`: 2
  - `goimports`: 52
  - `revive`: 11

`user-service` 的 depguard failures 均在 auth domain：

- `user-service/internal/features/auth/domain/rediskeys.go` 导入 `github.com/aegiscore/common/runtime/config`。
- `user-service/internal/features/auth/domain/rediskeys_test.go` 导入 `github.com/aegiscore/common/runtime/config`。

这些 findings 是本变更要清理的基线；实施时应重新运行 lint，以最新输出为准。

## Implementation Approach

### 1. Mechanical Formatting And Imports

先处理无行为风险的格式化问题：

- 对 lint 输出涉及的 Go 文件运行 `gofmt -w`。
- 对 lint 输出涉及的 Go 文件运行 `goimports -w`，并保持 `github.com/aegiscore` 作为本地 import prefix。

实施时可以先用 `golangci-lint run --fix ./...` 评估自动修复结果，但最终仍要审查 diff，避免 generated code 或无关文件被改动。Ent 生成代码仍应依赖 `.golangci.yml` exclusions，不应手动编辑。

### 2. Common Module Findings

`common/` 的非格式类问题按规则类型处理：

- `errcheck`：对 `Close` 等清理操作显式处理错误，或在 defer 中用 `_ = closer.Close()` 表达有意忽略。测试/容器 helper 如果清理失败不影响主要断言，应使用明确忽略，不要让错误悄悄未处理。
- `govet` inline warnings：将 `reflect.Ptr` 等可直接使用的常量内联，不保留无收益的局部常量。
- `revive` unused parameter：将确认未使用的参数改为 `_`，或删除参数；对 public API 或函数签名需要保持兼容时优先使用 `_`。
- `revive` blank import：给必要的 driver registration blank import 添加说明注释。
- `revive` context-as-argument：如果函数不是稳定外部 API，调整为 `context.Context` 第一参数；如果为现有 helper API 且改动面过大，可评估是否用有说明的局部排除，但优先小范围修正。
- `staticcheck` nil context：测试中用 `context.TODO()` 或显式覆盖 nil 语义；如果函数需要支持 nil，则测试应表达清楚并避免触发 SA1012。

修复后运行 `make lint-common`，直到 clean。

### 3. User Service Depguard Fix

Auth domain 的 `RedisKeyBuilder` 当前从 `*config.Config` 读取 app name，因此 domain 依赖 runtime config。按架构规则，config 读取属于 runtime/application/infrastructure 边界，domain 只能处理纯值。

推荐调整：

- 将 domain constructor 改为接收 plain string，例如 `NewRedisKeyBuilder(appName string) RedisKeyBuilder`。
- 在 constructor 内继续 `strings.TrimSpace(appName)`，保留现有空白裁剪行为。
- 在允许依赖 runtime config 的外层 provider 或 infrastructure wiring 中从 `config.Config` 提取 `cfg.App.Name`，再传入 domain constructor。
- 更新 tests，domain test 不再导入 `common/runtime/config`，只传 plain string。

如果 Fx provider 需要保留对 config 的自动注入，应新增 feature-local provider 函数放在 `user-service/internal/features/auth/fx.go` 或允许的组装层中，例如从 `*config.Config` 返回 `authdomain.RedisKeyBuilder`。不要把 config-aware constructor 放回 domain 包。

### 4. User Service Revive And Imports

`user-service/` 的 goimports findings 可机械修复。Revive findings 应保持最小行为改动：

- unused callback parameters 改为 `_`。
- 可直接 return 的 `if err := ...; err != nil { return err } return nil` 改为直接 `return ...`。
- `context.Context` 参数顺序按 revive 建议调整，同时更新调用点。
- exported function 返回 unexported type：优先评估是否将 return type 调整为 application port/interface 或导出 concrete type。若这些 constructors 仅由 Fx 使用且现有模式可接受，也可以在规则层面另开设计，但本变更不应简单用 `nolint` 掩盖多个相同问题。推荐在最小范围内导出 store concrete types 或让 constructor 返回消费侧 port。
- blank import 添加说明注释。

修复后运行 `make lint-user-service`，直到 clean。

## CI Gate Change

当本地 `make lint` 通过后，更新 `.github/workflows/lint.yml`：

- 删除 `continue-on-error: true`。
- 更新文件顶部注释，去掉“报告型检查”表述。
- 更新 step 注释，说明 lint 已作为阻断检查运行。

不需要新增 workflow。现有 matrix 已分别覆盖 `common` 和 `user-service`，并使用 `--config ../.golangci.yml ./...`，符合 workspace 结构。

## Documentation Updates

更新 `docs/GO_LINT_AUTOMATION.md`：

- 将“是否阻断合并”改为当前 lint workflow 已硬失败。
- 删除或改写首次验证基线中的旧 findings 数量，避免文档继续声称存在待清理存量问题。
- 删除 depguard 历史违规快照，或标记为已清理并说明新验证命令。
- 保留分阶段治理策略作为未来新增规则或历史分支合并时的通用策略，但不要让读者误以为当前 mainline 仍允许 lint failure。
- 保留 `golangci-lint` 安装、执行目录和 depguard 排查说明。

如 `docs/DEVELOPMENT.md` 中有“提交前建议”表述，可保持不变；如果新增“CI 会阻断 lint failure”说明，应与 `GO_LINT_AUTOMATION.md` 一致。

## Verification Strategy

按顺序运行：

```bash
golangci-lint config verify --config .golangci.yml
make lint-common
make lint-user-service
make lint
cd user-service && golangci-lint run --config ../.golangci.yml --enable-only depguard ./...
```

如果修复涉及函数签名或 constructor wiring，再运行相关测试：

```bash
make test-common
make test-user-service
```

至少应重点运行：

- `cd user-service && go test ./internal/features/auth/...`
- `cd common && go test ./...`

最后检查：

```bash
git diff -- .github/workflows/lint.yml .golangci.yml docs/GO_LINT_AUTOMATION.md docs/DEVELOPMENT.md common user-service
```

确认没有 Ent generated code、migration、Swagger 或 OpenSpec/OPSX 工件的无关改动。

## Risks

- 一次性 goimports/gofmt 可能触碰较多文件。缓解方式是只格式化 lint 输出涉及的文件，分模块验证。
- 为修复 revive 而调整 constructor return type 可能影响 Fx wiring。缓解方式是优先选择局部、类型安全的变更，并运行 user-service tests。
- depguard 违规若用 `nolint` 绕过，会破坏本变更目标。缓解方式是把 config 依赖移动到允许层，并用 depguard-only lint 验证。
- 移除 `continue-on-error` 前如果 lint 未清零，会导致 CI 立即阻断所有 PR。缓解方式是本地 `make lint` clean 后再改 workflow。
