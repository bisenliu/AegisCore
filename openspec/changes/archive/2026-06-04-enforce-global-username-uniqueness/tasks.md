## 1. User Creation Normalization

- [x] 1.1 在创建用户请求进入持久化前完成 `username` 空白裁剪和小写规范化，并保持 `nickname` 仅做展示名校验。
- [x] 1.2 移除创建流程中的 `ExistsByUsername` 或等价用户名存在性预查。
- [x] 1.3 确保创建成功响应返回小写规范化后的 `username`，并继续返回 `user_id`、`nickname`、`status`、`created_at`、`updated_at`。

## 2. Repository And Error Mapping

- [x] 2.1 更新 repository 创建实现，使 Ent/PostgreSQL 唯一约束错误转换为用户领域 `ErrUserAlreadyExists`。
- [x] 2.2 更新 service 错误映射，使 `ErrUserAlreadyExists` 转换为 HTTP 409、业务码 `40000` 和用户已存在文案。
- [x] 2.3 增加或调整测试覆盖并发/重复创建、大小写重复用户名、软删除用户名仍冲突的路径。

## 3. Database Schema And Migration

- [x] 3.1 更新 Ent `User` schema，确保 `username` 为全表唯一，`nickname` 不唯一，且不使用 `deleted_at IS NULL` partial unique index 释放用户名。
- [x] 3.2 在 `user-services` 模块运行 `go generate ./ent`，不得手写 `user-services/ent/` 生成代码。
- [x] 3.3 在 `user-services/` 运行 Atlas migration diff，审查 SQL 是否表达全表 `UNIQUE(username)` 和必要的数据冲突处理说明。
- [x] 3.4 如人工修改 migration SQL，运行 `atlas migrate hash --dir file://migrations` 并执行 migration validate 脚本。

## 4. Verification

- [x] 4.1 运行 `gofmt` 格式化修改过的 Go 文件。
- [x] 4.2 在 `common/` 和 `user-services/` 分别运行 `go test ./...`。
- [x] 4.3 验证创建用户成功、参数错误、重复用户名 409、软删除用户名重复 409 和内部错误不泄露数据库细节。
- [x] 4.4 确认所有业务引用用户身份的实现仍使用外部 `user_id`，未新增以 `username`、`nickname` 或内部 `id` 作为跨业务引用键的路径。
