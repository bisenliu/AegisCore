# Tasks

## Implementation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 和 `docs/TESTING.md`，确认当前长期规则与最终目录结构的差异。
- [x] 扫描目标文档中的旧目录、旧流程和过期结构引用：
  - `user-services/`、`./user-services`、`/app/user-services`
  - `OPSX`、`opsx`、`OpenSpec`、`openspec`
  - `capability`、`Capability` 或 capability map 类结构说明
- [x] 更新 `AGENTS.md`，确保仓库入口规则使用 `common/`、`user-service/`、`deployments/`，并保持当前 feature-first、分层依赖、迁移和生成代码规则。
- [x] 更新 `docs/ARCHITECTURE.md`，明确 module boundaries、runtime flow、HTTP request flow、feature-first organization、dependency rules、common organization、database migrations 和 generated code。
- [x] 更新 `docs/DEVELOPMENT.md`，确保 workspace layout、常用命令、配置、coding conventions、migration 流程、adding features 和 shared code 规则与当前结构一致。
- [x] 更新 `docs/TESTING.md`，确保测试入口、测试关注点、外部依赖、生成代码、迁移校验和 change verification 与当前结构一致。
- [x] 将服务内 data access 术语统一为 `infra adapter` 或具体 `infra/postgres`、`infra/redis`，避免继续表达为横向 Repository 层目录规则。
- [x] 修正长期规则文档中的旧 `user-services` 目录或 module path 引用；保留确属运行时稳定标识的 `aegiscore-user-services`，并按需说明它不是目录名。
- [x] 删除或修正把 OPSX/OpenSpec 描述为当前工作流的内容；只保留“不要新增 `openspec/` 或 `docs/opsx/`”这类禁止性规则。
- [x] 删除或替换过期 capability map 引用，改以 `AGENTS.md` 和 `docs/ARCHITECTURE.md` 作为结构规则入口。
- [x] 确认文档没有引导新增横向 `internal/controller`、`internal/service`、`internal/repository`、`internal/api`、`internal/domain` 或默认 `internal/shared`。
- [x] 确认文档没有要求移动代码、改业务行为、改配置 key、改数据库 schema 或改生成代码。

## Verification

- [x] 运行 `rg -n "user-services/|\\./user-services|/app/user-services" AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md`，确认无旧目录路径结果。
- [x] 运行 `rg -n "OPSX|opsx|OpenSpec|openspec" AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md`，确认只保留禁止性规则或无结果。
- [x] 运行 `rg -n "capability|Capability" AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md`，确认无过期 capability map 或旧结构规则。
- [x] 检查 `AGENTS.md` 与 `docs/ARCHITECTURE.md` 中的目录边界、feature-first 分层和依赖规则一致。
- [x] 检查 `docs/DEVELOPMENT.md` 与 `docs/TESTING.md` 的命令和验证流程与根目录 Makefile、`common/`、`user-service/` 一致。
- [x] 检查数据库迁移规则仍明确要求 Ent schema + Atlas migration，不允许运行时 `client.Schema.Create(ctx)` 修改 schema。
- [x] 检查生成代码规则仍明确禁止手写 `user-service/ent/` 生成代码。
- [x] 检查 `git diff -- AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md`，确认只有文档规则整理，没有代码、迁移或部署行为变更。

## Review Notes

- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。
- [x] 确认没有移动、重命名或删除任何代码目录。
- [x] 确认运行时稳定标识 `aegiscore-user-services` 未被误改。
- [x] 确认这些文档足以作为后续结构重构的唯一规则来源。
