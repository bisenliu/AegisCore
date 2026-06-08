# User Services Ent

`user-services/ent/` 包含用户服务的 Ent schema 源文件、Ent 代码生成入口，以及 repository 层使用的 Ent 生成代码。

## 生成代码边界

以下路径由开发者维护：

- `schema/`：Ent schema 源文件和 schema 聚合包。
- `generate.go`：Ent codegen 的 `go generate` 入口。

除 `schema/` 和 `generate.go` 外，`user-services/ent/` 下其他文件和目录均属于 Ent 生成代码。这包括顶层生成 Go 文件，例如 `client.go`、`ent.go`、`mutation.go`、`tx.go`、各 Entity 的操作文件，以及 `enttest/`、`hook/`、`migrate/`、`predicate/`、`runtime/` 和 Entity 查询包等生成目录。

不要手动修改生成文件。任何生成代码变更都必须来自 schema 源文件或 codegen 配置变更，并通过重新生成 Ent 代码产生。

## 重新生成 Ent 代码

在 `user-services/` 模块目录运行 Ent 代码生成：

```bash
go generate ./ent
```

提交前需要审查生成代码 diff。如果 schema 或 codegen 输入没有变化，生成文件不应需要手动修改。

## 新增 Entity Schema

新增一个 Entity Schema 时：

1. 按现有包组织在 `user-services/ent/schema/` 下新增 schema 源文件。
2. 通过根 `user-services/ent/schema` 包暴露新的 schema，使 Ent codegen 和 Atlas 能从稳定 schema source 读取。
3. 在 schema 源文件中定义字段、索引、边、注释、默认值和约束。不要通过编辑生成文件表达这些变更。
4. 在 `user-services/` 下运行 `go generate ./ent`。
5. 如果 schema 变更会改变数据库结构，在 `user-services/` 下运行 `./scripts/migrate-diff.sh <migration-name>` 生成 migration。
6. 提交前审查 `user-services/migrations/` 下生成的 SQL，以及更新后的 `user-services/migrations/atlas.sum`。
7. 如果手动调整生成的 SQL，在 `user-services/` 下运行 `atlas migrate hash --dir file://migrations`，然后运行 `./scripts/migrate-validate.sh`。

Entity Schema 变更应聚焦持久化 schema 语义。仅调整 schema 组织结构且不改变字段、索引、注释、默认值或约束时，不应新增 SQL migration。
