# Tasks

## Implementation

- [x] 阅读 `AGENTS.md` 和 `docs/ARCHITECTURE.md` 的 dependency rules，确认 depguard deny list 与长期架构规则一致。
- [x] 检查当前 `.golangci.yml` 的 `golangci-lint` v2 配置结构，保留现有 `standard`、`revive`、formatter 和 generated-code exclusions。
- [x] 在 `.golangci.yml` 的 `linters.enable` 中新增 `depguard`。
- [x] 在 `.golangci.yml` 的 `linters.settings.depguard.rules` 中新增 `feature-domain` 规则，禁止 domain import Gin、Ent、Redis、runtime config、logger、HTTP response envelope、application ports 和 infrastructure adapter。
- [x] 新增 `feature-application` depguard 规则，禁止 application import Gin、Ent、Redis 和 `common/http/binding`。
- [x] 新增 `feature-transport-http` depguard 规则，禁止 HTTP transport import Ent、Redis 和 SQL/pgx package。
- [x] 新增 `feature-transport-grpc` depguard 规则，禁止未来 gRPC transport import Ent、Redis、SQL/pgx、Gin、HTTP response envelope 和 external integration adapter。
- [x] 新增 `feature-infrastructure` depguard 规则，禁止 infrastructure import Gin 和 HTTP response envelope。
- [x] 新增 `integration-boundary` depguard 规则，禁止 integration import Gin response、Ent 和 feature infrastructure adapter。
- [x] 为每条 deny entry 添加清晰 `desc`，让 lint 输出能说明违反的架构边界。
- [x] 运行 `golangci-lint config verify --config .golangci.yml`，确认配置可读取。
- [x] 运行 `golangci-lint linters --config .golangci.yml`，确认 `depguard` 已启用。
- [x] 更新 `docs/GO_LINT_AUTOMATION.md` 的示例配置和完整策略，说明 depguard 的架构边界、运行命令和排查方式。
- [x] 如果启用 depguard 后存在历史违规，按 domain、application、transport、infrastructure 和 integration 分组记录分阶段治理清单。
- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。

## Verification

- [x] 运行 `make lint-common`，确认 common module lint 入口仍可执行。
- [x] 运行 `make lint-user-service`，确认 user-service module lint 入口可执行并加载 depguard。
- [x] 如果 `make lint-user-service` 因历史 depguard 违规失败，确认所有违规已被列入治理清单，且不混入无关历史 lint 修复。
- [x] 如发现 depguard file pattern 未覆盖目标目录，补充或修正 pattern 后重新运行配置验证和 user-service lint。
- [x] 检查 `docs/GO_LINT_AUTOMATION.md`，确认历史 lint 治理策略仍明确“不一次性清理所有历史问题”。
- [x] 检查 `git diff -- .golangci.yml docs/GO_LINT_AUTOMATION.md docs/changes/enforce-architecture-with-depguard`，确认变更范围只包含 lint 配置、lint 文档和本 change artifacts。

`make lint-common` 已运行，入口可执行并加载根配置；当前失败来自既有 `errcheck`、`gofmt`、`goimports`、`govet`、`revive` 和 `staticcheck` findings，未出现 depguard 违规。

`make lint-user-service` 已运行，入口可执行并加载 depguard；当前失败包含已记录的 2 个 depguard 历史违规：

- `user-service/internal/features/auth/domain/rediskeys.go` 导入 `github.com/aegiscore/common/runtime/config`。
- `user-service/internal/features/auth/domain/rediskeys_test.go` 导入 `github.com/aegiscore/common/runtime/config`。

其余 `user-service` lint findings 为既有 `goimports` 和 `revive` 问题，本变更未批量修复。

## Review Notes

- [x] 确认 depguard 规则没有误伤 `user-service/internal/providers`、`user-service/internal/router`、`common/http` 或 Ent generated code。
- [x] 确认 application 层仍可使用 `common/security` 和 `common/validation`。
- [x] 确认 transport/http 仍可使用 Gin、`common/http/response` 和 `common/contract/response`。
- [x] 确认 infrastructure/postgres 仍可使用 Ent 和 SQL，infrastructure/redis 仍可使用 Redis client。
- [x] 确认 integration adapter 仍可依赖外部 SDK/client、feature application ports、domain 和 common runtime/security primitives。
- [x] 确认文档没有恢复 OpenSpec/OPSX 工作流。
