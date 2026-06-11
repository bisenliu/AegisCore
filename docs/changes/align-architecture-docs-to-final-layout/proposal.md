# Align architecture docs to final layout

## What

将仓库入口文档和开发文档统一对齐到当前最终目录结构，使 `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 和 `docs/TESTING.md` 成为后续结构治理和重构的稳定规则来源。

包括：

- 统一文档中的服务目录、Go module 和命令说明为 `user-service`。
- 明确 `common/`、`user-service/`、`deployments/` 的边界和所有权。
- 补齐或校准 feature-first 结构、依赖方向、路由注册、Fx module、迁移、Ent 生成代码和共享代码准入规则。
- 删除或修正旧的 `user-services` 目录引用、OPSX/OpenSpec 工作流引用，以及过期 capability map 或类似旧结构说明。
- 同步开发和测试文档中的验证入口，确保命令从仓库根目录 Makefile 或明确模块目录执行。

本变更只修改文档，不移动代码、不改变 Go module、运行时配置、HTTP API、数据库 schema、migration、生成代码或业务行为。

## Why

仓库已经完成从旧目录命名和旧结构说明到当前 `common` + `user-service` + `deployments` 布局的迁移，但长期规则文档需要成为唯一可信来源。若文档中继续混杂旧目录、旧流程或过期结构图，后续 AI 代理和协作者会更容易在错误位置新增代码，重新引入横向 service/repository/controller 包，或误以为需要恢复 OpenSpec/OPSX 工件。

将入口文档、架构文档、开发文档和测试文档统一到最终布局，可以让后续 feature 扩展、共享能力抽取、迁移生成和测试验证都有一致依据，减少结构漂移。

## Scope

包括：

- 更新 `AGENTS.md` 的仓库入口规则、目录说明、关键入口、开发命令和禁止事项。
- 更新 `docs/ARCHITECTURE.md` 的模块边界、运行时流程、HTTP request flow、feature-first 结构、依赖规则、common 组织、迁移规则和生成代码规则。
- 更新 `docs/DEVELOPMENT.md` 的 workspace layout、常用命令、coding conventions、迁移流程、增加 feature 和共享代码规则。
- 更新 `docs/TESTING.md` 的测试入口、测试关注点、生成代码验证、迁移验证和 change verification。
- 扫描上述文档，修正旧 `user-services` 目录路径、旧 OPSX/OpenSpec 流程说明、过期 capability map 或旧目录结构引用。
- 保留确实属于运行时稳定标识的 `aegiscore-user-services` 示例或名称，并在需要时说明它不是目录名。

不包括：

- 不移动或重命名任何代码目录。
- 不修改 Go import path、module path、CLI 名称、配置 key、JWT issuer、日志文件基准名或 Redis key prefix。
- 不修改 `user-service/ent/` 生成代码、Ent schema、Atlas migration 或 Swagger 产物。
- 不新增 `openspec/`、`docs/opsx/` 或其他 OpenSpec/OPSX 工件。
- 不重写历史 change 记录，除非其中内容被当前文档直接引用为长期规则。
- 不改变业务行为、HTTP API、数据库结构、容器入口或部署行为。

## Acceptance Criteria

- `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 和 `docs/TESTING.md` 均使用当前目录 `user-service` 表达服务代码位置。
- 上述长期规则文档中不存在旧目录路径 `user-services/` 或旧 module path 引用。
- 上述长期规则文档不再把 OPSX/OpenSpec 作为当前开发工作流；如提到这些名称，只用于禁止重新新增相关工件。
- 上述长期规则文档不存在过期 capability map 或旧横向目录结构说明。
- 文档明确 `common/`、`user-service/`、`deployments/` 的边界。
- 文档明确服务内 feature-first 结构和 `api/app/domain/transport/http/infra/*/module.go` 分层职责。
- 文档明确依赖方向：domain 不依赖 transport/infra/runtime，app 不依赖 Gin/Ent/Redis，transport 不依赖 SQL/Redis，infra 不返回 HTTP response。
- 文档明确数据库迁移必须通过 Ent schema + Atlas migration 管理，不通过运行时 `client.Schema.Create(ctx)` 修改 schema。
- 文档明确 `user-service/ent/` 生成代码不得手写，修改 schema 后需要重新生成。
- 文档可以作为后续结构重构的唯一规则来源，且与当前仓库目录结构一致。
