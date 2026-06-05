## Context

用户服务当前已将 Ent `User` schema 通过根 `user-services/ent/schema` package 聚合，并把字段与索引拆到 `user-services/ent/schema/user` 子包。该子包包名为 `user`，与 Ent codegen 生成的查询包 `user-services/ent/user` 同名。手写 schema 聚合入口导入 `ent/schema/user`，而 PostgreSQL repository 导入 `ent/user`，两者路径接近且包名相同，容易在 IDE 自动导入、人工阅读和代码搜索时混淆。

本次变更属于 Ent schema source 组织命名优化，边界位于 `database-schema-migrations` 能力。它不应触碰 HTTP controller/service/repository 分层，不应改变用户资料 API、运行时配置、Redis/PostgreSQL 初始化、错误映射或响应信封。

## Goals / Non-Goals

**Goals:**

- 消除 `ent/schema/user` 与 `ent/user` 在 Go 包名上的歧义。
- 保持根 `schema` package 作为 Ent codegen 和 Atlas schema source 的稳定入口。
- 保持 `User` 字段、索引、字段注释、数据库表结构和 migration 历史不变。
- 通过 `go generate ./ent` 验证 schema 包重命名后 Ent 生成路径仍可用。

**Non-Goals:**

- 不修改 Ent 生成查询包 `user-services/ent/user` 的名称或内容。
- 不手写 `user-services/ent/` 下生成代码。
- 不新增或修改 SQL migration、`atlas.sum` 或数据库表结构。
- 不改变用户查询、创建、认证、响应或 repository 行为。

## Decisions

1. 将 schema 子包命名为 `userschema`，而不是继续使用 `user`。

   理由：`userschema` 明确表达该包承载 Ent schema source 的用户字段与索引定义，能与 Ent codegen 的 `ent/user` 查询包形成稳定区分。相比 `userfields`，`userschema` 更适合未来同时承载字段、索引、mixin 或 annotation 等 schema 相关定义。

2. 保留根 `user-services/ent/schema/user.go` 中的 `type User struct { ent.Schema }` 聚合入口。

   理由：Ent codegen 和 Atlas schema source 已依赖根 `schema` package 发现 `User` schema。重命名内部子包即可降低歧义，无需改变生成入口或 Atlas 配置。

3. 仅移动和重命名 schema source 子包，不修改 `Fields()` 与 `Indexes()` 返回内容。

   理由：本次目标是可维护性命名优化，不是数据库语义变更。只要字段、索引、注释、默认值和约束保持一致，就不应生成新的 SQL migration。

4. 使用生成流程验证，不手动编辑生成代码。

   理由：仓库规则要求修改 Ent schema 后在 `user-services` 模块运行 `go generate ./ent`，并禁止手写 `user-services/ent/` 下生成代码。

## Risks / Trade-offs

- [Risk] Go import 路径更新遗漏导致编译或 Ent 生成失败 -> Mitigation: 更新根 schema 聚合入口 import，并运行 `go generate ./ent` 与 `go test ./...` 验证。
- [Risk] 纯目录重命名被误认为数据库结构变更 -> Mitigation: 明确不修改字段、索引、注释和默认值，并确认不新增 SQL migration 或修改 `atlas.sum`。
- [Risk] 未来 schema 子包继续出现与生成包同名的问题 -> Mitigation: 在 `database-schema-migrations` delta spec 中新增命名约束，要求分类子包名称显式表达 schema source 语义。
