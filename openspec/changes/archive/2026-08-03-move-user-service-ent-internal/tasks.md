## 1. Ent 目录迁移

- [x] 1.1 创建 `user-service/internal/persistence/ent/`，将 `user-service/ent/schema/` 和 `user-service/ent/generate.go` 迁入新目录。
- [x] 1.2 删除旧 `user-service/ent/` 生成物，不保留根级兼容包、别名、shim 或双路径支持。
- [x] 1.3 在新目录执行 Ent 生成，确保生成物落在 `github.com/aegiscore/user-service/internal/persistence/ent` 及其子包。

## 2. 内部引用收敛

- [x] 2.1 更新 `user-service/internal/providers/` 中 Ent client、插件、metrics 和 tracing 的 import 路径。
- [x] 2.2 更新 `user-service/internal/features/*/infrastructure/` 中 Ent client、entity、predicate 和 `enttest` 的 import 路径。
- [x] 2.3 更新 `user-service/cmd/` 的 RBAC 依赖组装和相关测试 import 路径。
- [x] 2.4 更新 `user-service/scripts/atlas-schema/`，使 Atlas schema helper 只导入 `internal/persistence/ent/migrate`。
- [x] 2.5 全仓搜索并清除 `github.com/aegiscore/user-service/ent` 及其子包 import，确认 `user-service/ent/` 根级目录不存在。

## 3. 架构门禁

- [x] 3.1 更新 `user-service-architecture-lint` 对应实现，拒绝 `github.com/aegiscore/user-service/ent` import 和根级 `user-service/ent` 目录回归。
- [x] 3.2 补充或更新架构 lint 测试，覆盖旧 Ent import path 和旧目录回归失败场景。

## 4. 生成与验证

- [x] 4.1 运行 `make user-service-generate`，检查 Ent 生成物无 drift 且未重建 `user-service/ent/`。
- [x] 4.2 运行 `make user-service-migrate-validate`，确认迁移 SQL、Atlas hash 和 schema helper 仍一致。
- [x] 4.3 运行 `make user-service-architecture-lint`，确认架构边界检查通过。
- [x] 4.4 运行 `make user-service-test`，确认 provider、feature infrastructure、RBAC CLI 和测试均使用新 Ent 路径。
- [x] 4.5 使用 `git diff --exit-code` 或等价方式检查生成物 drift；如存在本 change 预期 diff，先审查并暂存预期代码、生成物、OpenSpec 和文档变更。
- [x] 4.6 暂存本次预期变更后运行 `make lint` 和 `make verify`，验证通过后再将本 change 任务标记完成。
