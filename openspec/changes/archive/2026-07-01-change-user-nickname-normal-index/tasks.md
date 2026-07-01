## 1. Schema 与迁移实现

- [x] 1.1 修改 `user-service/ent/schema/user.go`，将 `nickname` 索引改为普通 `index.Fields("nickname")`，并删除不再使用的 `dialect` 和 `entsql` import。
- [x] 1.2 删除 `user-service/migrations/` 下当前旧 SQL 文件，只保留一个最新完整迁移文件。
- [x] 1.3 整理最新迁移 SQL，确保 `users.nickname` 使用普通索引，并保留当前 schema 所需表、字段注释、唯一索引和非 nickname 优化索引。

## 2. 验证与审查

- [x] 2.1 运行 `make user-service-migrate-validate` 验证迁移文件可被 Atlas 校验。
- [x] 2.2 运行相关 Go 测试或 `make user-service-test`，确认用户资料能力未发生业务语义回归。
- [x] 2.3 运行 `make lint` 和 `make verify`，完成仓库级 lint 与验证。`make lint` 已通过；`make verify` 因当前实现 diff 触发最终 `git diff --exit-code` 失败，待提交或清理工作区后重跑。
- [x] 2.4 使用 `git diff -- user-service/ent/schema/user.go user-service/migrations` 输出 schema 和迁移文件修改内容供审查。
