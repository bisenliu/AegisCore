## Context

用户服务的 Ent 目录同时承担三类职责：`generate.go` 定义 codegen 入口，`schema/` 存放可维护的 schema source，其余文件和目录主要由 Ent 生成。现有 `docs/DEVELOPMENT.md` 已包含 `go generate ./ent` 命令和不要手写生成代码的原则，但缺少就近 README 来解释 `user-services/ent/` 的目录边界和新增 Entity Schema 时的完整流程。

本变更是文档与规格补充，不涉及 controller/service/repository 分层、运行时依赖、HTTP 响应契约、Redis/PostgreSQL 连接、Ent schema 字段或 Atlas migration 历史。

## Goals / Non-Goals

**Goals:**

- 在 `user-services/ent/README.md` 说明哪些内容属于开发者维护的 schema source 与 codegen 入口，哪些属于生成代码。
- 记录重新生成代码的标准命令：在 `user-services/` 模块执行 `go generate ./ent`。
- 明确警告不要手动修改生成文件。
- 说明新增 Entity Schema 的基本流程，包括新增 schema source、运行 Ent codegen、按需生成并审查 Atlas migration、提交相关文件。
- 在 `docs/DEVELOPMENT.md` 加入对该 README 的引用。

**Non-Goals:**

- 不修改 Ent schema、生成代码、migration SQL 或 `atlas.sum`。
- 不新增运行时 API、配置项、依赖注入 provider 或数据库连接逻辑。
- 不改变 Atlas migration 脚本、Ent codegen 参数或 schema 包组织结构。
- 不为当前不存在的业务实体设计字段或表结构。

## Decisions

- 决策：将详细说明放在 `user-services/ent/README.md`，并从 `docs/DEVELOPMENT.md` 引用。
  备选方案是在 `docs/DEVELOPMENT.md` 内完整展开所有说明。选择就近 README 是因为开发者浏览 Ent 目录时能直接看到生成代码边界，同时开发总览仍保留跳转入口，避免主开发文档过长。

- 决策：以排除法定义生成代码边界，即 `schema/` 和 `generate.go` 由开发者维护，`user-services/ent/` 下其他文件和目录视为 Ent 生成代码。
  备选方案是逐一列出当前所有生成文件和目录。选择排除法是因为 Ent 生成结果会随 schema 和 Ent 版本变化，排除法更稳定，也符合本次明确要求。

- 决策：新增 Entity Schema 流程只描述通用步骤和注意事项，不加入具体实体示例代码。
  备选方案是给出完整 schema 示例。选择通用流程是因为本变更不决定任何新业务实体，不应引入可能被误用的字段、索引或表名约定。

- 决策：规格 delta 归属 `database-schema-migrations`。
  备选方案是新增单独的 Ent codegen 文档能力。选择修改既有能力是因为 Ent schema、生成代码和 Atlas migration 已是该能力的长期工作流组成部分，本变更只补充文档化要求。

## Risks / Trade-offs

- 文档说明与未来 Ent 目录结构变化不一致 -> 使用排除法描述生成代码边界，并要求 schema 或 codegen 入口变化时同步更新 README。
- 开发者只阅读 `docs/DEVELOPMENT.md` 而忽略 Ent README -> 在开发文档的命令与编码规范附近加入引用，使入口更明显。
- 新增 Entity Schema 说明被误解为可以跳过 Atlas migration -> README 将明确数据库结构变更后需要生成、审查并提交 migration 与 `atlas.sum`。
- 仅文档变更难以通过测试验证 -> 通过文件存在性和内容审查验证；不运行会改写 Ent 生成代码或 migration 的命令。
