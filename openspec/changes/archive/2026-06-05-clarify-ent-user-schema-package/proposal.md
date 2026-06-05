## Why

当前用户服务将手写 Ent schema 聚合入口 `user-services/ent/schema/user.go` 指向子包 `github.com/aegiscore/user-services/ent/schema/user`，而仓库内同时存在 Ent 生成查询包 `github.com/aegiscore/user-services/ent/user`。两个包名均为 `user`，在 IDE 自动导入、人工阅读和代码搜索时容易混淆 schema 子包与 Ent 生成查询包，增加误改 schema 或错误 import 的概率。

## What Changes

- 将 `user-services/ent/schema/user` schema 子包重命名为更明确的名称，例如 `userschema` 或 `userfields`。
- 更新根 `user-services/ent/schema` 聚合入口的 import 和调用，使 Ent codegen 与 Atlas schema source 仍通过根 `schema` package 读取 `User` schema。
- 保持 `User` 字段、索引、字段注释、表结构、migration 历史和运行时 API 行为不变。
- 非目标：不修改 Ent 生成查询包 `user-services/ent/user`，不手写 `user-services/ent/` 下生成代码，不新增数据库 migration。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `database-schema-migrations`: 明确 Ent schema 领域拆分中的子包命名必须避免与 Ent 生成包混淆，同时保持根 `schema` package、Ent 生成和 Atlas migration 工作流稳定。

## Impact

- 受影响代码：`user-services/ent/schema/user.go`、`user-services/ent/schema/user/schema.go` 及对应目录名。
- 参考冲突位置：`user-services/internal/repository/postgres/user_repository.go` 当前导入 Ent 生成包 `github.com/aegiscore/user-services/ent/user`。
- 外部 API、错误码、配置、数据库表结构和既有 migration 均不应发生变化。
- 实现后需在 `user-services` 模块运行 `go generate ./ent` 验证生成工作流，并确认不会因纯命名重构生成新的 SQL migration。
