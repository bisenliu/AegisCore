# Tasks

## Implementation

- [x] 重新运行 `make lint-common`，记录当前 `common/` lint findings。
- [x] 重新运行 `make lint-user-service`，记录当前 `user-service/` lint findings。
- [x] 对 lint 输出涉及的 `common/` Go 文件运行 `gofmt`/`goimports`，并审查格式化 diff。
- [x] 修复 `common/` 中 `errcheck` findings，显式处理或有意忽略 `Close` 错误。
- [x] 修复 `common/` 中 `govet` inline findings，移除无收益的 `reflect.Ptr` 等局部常量。
- [x] 修复 `common/` 中 `revive` findings，包括 unused parameter、blank import comment 和 context 参数顺序。
- [x] 修复 `common/` 中 `staticcheck` nil context findings。
- [x] 运行 `make lint-common`，确认共享模块 lint clean。
- [x] 将 `auth/domain.RedisKeyBuilder` 改为接收 plain app name，不再导入 `common/runtime/config`。
- [x] 在允许的 Fx/provider 组装层从 `*config.Config` 构造 `RedisKeyBuilder`，保持 Redis key prefix 行为不变。
- [x] 更新 `auth/domain/rediskeys_test.go`，不再导入 runtime config。
- [x] 对 lint 输出涉及的 `user-service/` Go 文件运行 `gofmt`/`goimports`，并审查格式化 diff。
- [x] 修复 `user-service/` 中 `revive` findings，包括 unused parameter、redundant if-return、context 参数顺序、constructor return type 和 blank import comment。
- [x] 运行 `cd user-service && golangci-lint run --config ../.golangci.yml --enable-only depguard ./...`，确认 depguard clean。
- [x] 运行 `make lint-user-service`，确认用户服务模块 lint clean。
- [x] 移除 `.github/workflows/lint.yml` 中的 `continue-on-error: true`，并更新报告型检查相关注释。
- [x] 更新 `docs/GO_LINT_AUTOMATION.md`，说明 lint workflow 已作为阻断门禁，移除旧存量问题快照或标记已清理。
- [x] 如需要，更新 `docs/DEVELOPMENT.md` 的 lint 说明，使本地与 CI 门禁语义一致。
- [x] 确认没有新增 `openspec/`、`docs/opsx/` 或其他已退役流程工件。

## Verification

- [x] 运行 `golangci-lint config verify --config .golangci.yml`。
- [x] 运行 `make lint-common`。
- [x] 运行 `make lint-user-service`。
- [x] 运行 `make lint`。
- [x] 运行 `cd user-service && golangci-lint run --config ../.golangci.yml --enable-only depguard ./...`。
- [x] 运行 `make test-common`。
- [x] 运行 `make test-user-service`。
- [x] 检查 `.github/workflows/lint.yml`，确认 lint step 没有 `continue-on-error`。
- [x] 检查 `docs/GO_LINT_AUTOMATION.md`，确认不再声明当前仓库仍有未清理 lint 基线。
- [x] 检查 `git diff`，确认变更范围不包含 Ent generated code、migration、Swagger 或 OpenSpec/OPSX 工件。

## Review Notes

- [x] 确认 depguard 规则没有被放松，也没有用无说明的 `nolint` 绕过。
- [x] 确认 auth domain 只接收 plain value，不读取 runtime config。
- [x] 确认 Redis key 输出保持与旧测试期望一致，包括 app name trim、空 app name 不加前缀和 Redis hash tag 格式。
- [x] 确认 lint workflow matrix 仍分别覆盖 `common` 和 `user-service`。
- [x] 确认文档没有恢复 OpenSpec/OPSX 工作流。
