# Design

## Overview

本变更是文档一致性整理，不改变代码。实现时以 `AGENTS.md` 和 `docs/ARCHITECTURE.md` 作为长期结构规则入口，再让 `docs/DEVELOPMENT.md` 和 `docs/TESTING.md` 对齐这些规则。

文档最终要表达一个清晰模型：

- 仓库根目录是 Go workspace，不是业务 Go module。
- `common/` 是跨服务稳定契约和基础能力。
- `user-service/` 是当前用户服务模块，拥有运行时、feature、Ent schema、Atlas migration 和 Swagger 文档。
- `deployments/` 只保存 Docker、Compose、Kubernetes 和 Helm 部署资产。
- 服务内代码按 feature-first 组织，不恢复横向 controller/service/repository/domain 根包。
- 结构规则以当前文档为准，不重新引入 OpenSpec/OPSX 工件。

## Documentation Model

### AGENTS.md

`AGENTS.md` 是 AI 代理和协作者的入口。它应保持短而可执行，包含：

- Quick Start：指向架构、开发、产品、测试和 lint 文档。
- Repository Shape：列出 `go.work`、`common/`、`user-service/`、当前 feature 和 `deployments/`。
- Key Entry Points：列出 CLI、Fx bootstrap、routes、router、feature module/controller/service/adapter、共享 runtime provider、Atlas 和 migration 目录。
- Current Feature Areas：列出用户资料、认证会话、HTTP runtime、共享基础设施、响应契约和数据库迁移。
- Development Commands：以根目录 Makefile 为优先入口。
- Change Workflow：要求结构规则变更同步更新 `AGENTS.md` 与 `docs/ARCHITECTURE.md`。
- Repository Rules：记录 feature-first、分层依赖、route/module 所有权、common 准入、`internal/shared` 准入、ports 所有权、validation、controller/service DTO 边界、Ent predicate 和 adapter 禁止事项。

`AGENTS.md` 不应包含详细教程或历史迁移说明；这些内容属于 `docs/DEVELOPMENT.md` 或历史 change 记录。

### docs/ARCHITECTURE.md

`docs/ARCHITECTURE.md` 是长期结构规则的完整版本。它应包含：

- Overview：说明 workspace、技术栈和规则来源。
- Module Boundaries：用表格明确 `common`、`user-service`、`deployments`。
- Runtime Flow：从 `cmd/main.go` 到 Fx、feature modules、routes 和 HTTP server lifecycle。
- HTTP Request Flow：从 middleware、route registration、controller、app service、infra adapter 到 response envelope。
- Feature-First Organization：定义 `api/`、`app/`、`domain/`、`transport/http/`、`infra/postgres/`、`infra/redis/`、`module.go`。
- Dependency Rules：用表格记录每层可以依赖和禁止依赖的内容。
- Common Organization：说明 `contract`、`runtime`、`http`、`security`、`validation`。
- Data Model：保留当前用户模型的高层说明。
- Infrastructure：说明配置、Redis/PostgreSQL named resources、Ent clients、日志和 trace-id。
- Database Migrations：说明 Ent schema + Atlas migration 流程、执行目标和禁止 runtime schema create。
- Generated Code：说明 `user-service/ent/` 生成代码边界。
- Current Constraints：列出当前 API 和运行时依赖限制。

该文档应避免出现过期 capability map 或旧 OPSX/OpenSpec 结构说明。若保留“不要新增 OpenSpec/OPSX 工件”的约束，应表述为当前仓库规则，而不是当前工作流。

### docs/DEVELOPMENT.md

`docs/DEVELOPMENT.md` 面向日常开发，应与架构规则一致：

- Prerequisites：Go/toolchain、golangci-lint、PostgreSQL/Redis、Atlas、配置样例。
- Workspace Layout：只列 `common/go.mod`、`user-service/go.mod` 和 `go.work`。
- Common Commands：以 `make help`、`make build`、`make test`、`make run-user-service`、`make lint`、`make generate`、`make migrate-*`、`make swagger-generate` 为主。
- Configuration：说明 YAML + `AEGISCORE_` 覆盖、named Redis/PostgreSQL 和当前用户服务声明的资源。
- Coding Conventions：应使用 feature-first 术语，避免继续称服务内 data access 为横向 Repository 层；可使用 “infra adapter” 表达。
- Database Migrations：以 `user-service/atlas.hcl`、`user-service/ent/`、`user-service/migrations/`、`user-service/scripts/` 为准。
- Adding Features / Shared Code：与 `docs/ARCHITECTURE.md` 的 feature-first 和 common 准入规则一致。

### docs/TESTING.md

`docs/TESTING.md` 面向验证。它应：

- 说明根目录 `make test` 是全仓库入口，并解释根目录不是 Go module。
- 保留模块级 `go test ./...` 入口：`common/` 与 `user-service/`。
- 将测试关注点按 controller、app service、infra adapter、middleware、config loader、runtime/infrastructure、logging 表达。
- 说明单元测试应隔离 Redis/PostgreSQL，集成测试需显式说明外部依赖。
- 说明 Ent schema 和 migration 变更的生成、SQL review、hash/validate 和禁止 runtime schema create。
- 说明 change verification 应按改动范围运行测试、生成、迁移校验、HTTP envelope 验证和启动流程验证。

## Reference Cleanup Rules

实现时使用文本扫描辅助，但不能做盲目替换。

### Old `user-services`

需要修正：

- 长期规则文档中的旧目录路径，例如 `user-services/`、`./user-services`、`/app/user-services`。
- 旧 Go module path 或 import path，如果出现在长期规则文档中。
- 旧开发命令中以旧目录作为执行目录的示例。

可以保留：

- 稳定运行时名称 `aegiscore-user-services`，例如 CLI `Use`、配置 `app.name`、JWT issuer、日志文件名示例、Redis key prefix 示例。
- 历史 change 记录中的旧迁移背景，除非该历史记录被当前长期规则文档引用为现行规范。

如长期规则文档中保留 `aegiscore-user-services`，应让上下文明确它是运行时标识，不是目录名或 module path。

### OPSX/OpenSpec

需要删除或修正：

- 把 `/opsx:*` 或 `openspec` 命令描述为当前 change workflow 的说明。
- 要求新增 `openspec/`、`docs/opsx/` 或 OpenSpec artifact 的说明。
- 旧 capability map 或 OpenSpec capability 目录引用。

可以保留：

- 简短的禁止性规则，例如“本仓库不再维护 OpenSpec/OPSX 工件，不要重新新增 `openspec/` 或 `docs/opsx/`。”

### Capability Map

需要删除或替换：

- 旧 capability map、capability package、capability spec、或按 capability map 查找结构规则的说明。
- 与当前 feature-first 布局冲突的旧结构索引。

替换方式：

- 用 `docs/ARCHITECTURE.md` 的 Module Boundaries、Feature-First Organization 和 Dependency Rules 作为结构索引。

## Implementation Approach

1. 扫描当前目标文档中的旧引用：

   ```bash
   rg -n "user-services/|\\./user-services|/app/user-services|OPSX|opsx|OpenSpec|openspec|capability|Capability" AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md
   ```

2. 按文档角色逐个修正内容：

   - 先更新 `AGENTS.md`，保证入口规则简洁且可执行。
   - 再更新 `docs/ARCHITECTURE.md`，保证长期规则完整。
   - 再更新 `docs/DEVELOPMENT.md`，保证日常命令和开发流程一致。
   - 最后更新 `docs/TESTING.md`，保证验证流程与当前结构一致。

3. 对每个文档中的 `user-services` 命中分类处理：

   - 目录、module、路径、命令语境：改为 `user-service`。
   - 运行时名称语境：保留，并按需补一句说明。
   - 历史 change 语境：不作为本次长期规则整理目标。

4. 对 OPSX/OpenSpec/capability map 命中分类处理：

   - 当前工作流描述：删除或改为当前 docs-native change 记录方式。
   - 禁止性规则：保留简短约束。
   - 旧结构来源：替换为 `AGENTS.md` + `docs/ARCHITECTURE.md`。

5. 统一术语：

   - 使用 `feature`、`app service`、`infra adapter`、`transport/http controller`。
   - 避免在当前服务内规则中继续使用横向 Repository 层作为目录结构概念。
   - 需要描述存储访问时，使用 `infra/postgres adapter` 或 `infra/redis adapter`。

## Compatibility

本变更是纯文档变更：

- 不影响 Go build、tests、lint、Swagger、Ent 生成或 Atlas migration。
- 不影响 `aegiscore-user-services` 这类运行时稳定标识。
- 不影响历史 change 记录的审计价值。
- 不改变根目录 Makefile、脚本、部署资产或代码目录。

## Verification Strategy

- 文件存在性：确认四个目标文档已更新。
- 文本扫描：
  - `rg -n "user-services/|\\./user-services|/app/user-services" AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md` 无结果。
  - `rg -n "OPSX|opsx|OpenSpec|openspec" AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md` 只保留禁止性规则或无结果。
  - `rg -n "capability|Capability" AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md` 无过期结构规则。
- 结构一致性检查：确认目标文档均指向 `common/`、`user-service/` 和 `deployments/`。
- 分层规则检查：确认 `api/app/domain/transport/http/infra/*/module.go`、dependency rules、ports 所有权、validation 和 controller/service DTO 边界均有明确说明。
- 迁移与生成规则检查：确认 Ent schema、Atlas migration、`client.Schema.Create(ctx)` 禁止项和生成代码禁止手写均有明确说明。
- 文档 diff review：确认没有改动代码、生成产物、迁移或部署行为。
