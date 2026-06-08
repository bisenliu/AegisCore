## Why

`user-services/ent/` 同时包含 Ent schema 源文件、codegen 入口和大量生成代码，当前文档只笼统要求不要手写生成代码，开发者缺少可直接查阅的目录边界与新增 Entity Schema 流程说明。

本变更补齐生成代码说明，降低误改生成文件、遗漏重新生成或误触发迁移历史变更的风险。

## What Changes

- 在 `user-services/ent/README.md` 新增 Ent 目录说明，明确除 `schema/` 和 `generate.go` 外的文件属于生成代码。
- 在 README 中说明重新生成命令为在 `user-services/` 模块执行 `go generate ./ent`。
- 在 README 中加入不要手动修改生成文件的警告。
- 在 README 中说明新增 Entity Schema 的基本流程和注意事项，包括修改 schema source、运行 Ent 生成、按需生成 Atlas migration、审查并提交相关文件。
- 在 `docs/DEVELOPMENT.md` 中增加到 `user-services/ent/README.md` 的引用，方便开发者从开发文档跳转查阅。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `database-schema-migrations`: 补充开发者必须能够查阅用户服务 Ent 生成代码边界与 Entity Schema 新增流程的文档要求。

## Impact

- 影响文档：`user-services/ent/README.md`、`docs/DEVELOPMENT.md`。
- 影响规格：更新 `database-schema-migrations` 的文档化要求。
- 不改变 HTTP API、错误码、运行时配置、数据库结构、Ent schema、生成代码或 Atlas migration 历史。
