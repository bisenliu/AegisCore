## ADDED Requirements

### Requirement: Document Ent generated code boundaries

系统 SHALL 为用户服务 Ent 目录提供就近文档，明确开发者可维护的 schema source 与 codegen 入口，并明确其余 Ent codegen 输出不得手动修改。

#### Scenario: Generated files are identifiable

- **WHEN** 开发者查看 `user-services/ent/README.md`
- **THEN** 文档 MUST 说明 `user-services/ent/schema/` 是 Ent schema source 目录
- **THEN** 文档 MUST 说明 `user-services/ent/generate.go` 是 `go generate` 入口
- **THEN** 文档 MUST 说明除 `schema/` 和 `generate.go` 外，`user-services/ent/` 下其他文件和目录属于 Ent 生成代码

#### Scenario: Regeneration command is documented

- **WHEN** 开发者需要刷新用户服务 Ent 生成代码
- **THEN** 文档 MUST 指示开发者在 `user-services/` 模块运行 `go generate ./ent`
- **THEN** 文档 MUST 警告开发者不要手动修改生成文件

#### Scenario: New entity schema workflow is documented

- **WHEN** 开发者需要新增一个 Entity Schema
- **THEN** 文档 MUST 说明应在 `user-services/ent/schema/` 下新增或聚合 schema source
- **THEN** 文档 MUST 说明新增或修改 schema source 后运行 `go generate ./ent`
- **THEN** 文档 MUST 说明数据库结构发生变化时需要通过 Atlas 生成并审查 `user-services/migrations/` 下的 SQL migration 和 `atlas.sum`
- **THEN** 文档 MUST 说明不得通过手动编辑生成代码表达 schema 变更

#### Scenario: Development guide links to Ent README

- **WHEN** 开发者阅读 `docs/DEVELOPMENT.md`
- **THEN** 开发文档 MUST 提供到 `user-services/ent/README.md` 的引用
- **THEN** 该引用 MUST 用于查阅 Ent 生成代码边界、重新生成命令和新增 Entity Schema 流程
