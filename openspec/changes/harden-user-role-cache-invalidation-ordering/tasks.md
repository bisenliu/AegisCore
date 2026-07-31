## 1. 实现准备

- [x] 1.1 复核 `user-service/internal/features/permission/infrastructure/casbin/user_role_resolver.go`、`user_role_cache.go` 和现有测试，确认 `RolesForUser`、`InvalidateUserRole`、`InvalidateAllUserRoles` 的调用路径和统计语义。
- [x] 1.2 选择 permission infrastructure 内部 generation 承载方式，确保不修改 `common/runtime/localcache` 公共 API，不改变 HTTP API、OpenAPI、数据库 schema 或部署资产。

## 2. 缓存失效顺序门禁

- [x] 2.1 在 user-role resolver 或同包私有 cache wrapper 中实现全量 generation 与 per-user generation token，load 开始时捕获 token，写入缓存前校验 token。
- [x] 2.2 调整 `InvalidateUserRole(userID)`，先提升该用户 generation/revision，再删除该用户缓存项，并保持 cache disabled 模式安全。
- [x] 2.3 调整 `InvalidateAllUserRoles()`，先提升全量 generation/revision，再清空缓存，并抑制所有全量失效前启动的旧 load 写回。
- [x] 2.4 确保 generation 过期、回源错误和 context 取消均不写入缓存，并让授权路径继续 fail-closed。
- [x] 2.5 保持 `RolesForUser` 命中、回源和 cache disabled 直接回源返回独立 `[]uuid.UUID`，保持 direct stats 的 `LoadSuccess` 与 `LoadError` 语义。

## 3. 测试覆盖

- [x] 3.1 补充单用户 cache miss 与用户角色 Add/Remove/Replace 写后失效并发测试，验证旧 load 在失效后完成时不能回填旧角色集合。
- [x] 3.2 补充 `InvalidateAllUserRoles` 并发测试，验证全量失效前开始的多个旧 load 均不能写回缓存。
- [x] 3.3 补充 generation 过期 load 的 fail-closed 测试，验证旧角色集合不会产生允许结果，后续请求能重新回源到最终状态。
- [x] 3.4 补充或更新 cache disabled 模式测试，验证直接回源、独立 slice、失效安全和回源错误 fail-closed。
- [x] 3.5 运行目标包测试和 race 测试，例如 `go test ./user-service/internal/features/permission/infrastructure/casbin` 与 `go test -race ./user-service/internal/features/permission/infrastructure/casbin`。

## 4. 架构与交付验证

- [x] 4.1 运行 `make user-service-architecture-lint`，确认未违反 common、shared、feature 分层和 RBAC 边界。
- [x] 4.2 检查生成物 drift；本变更预期不修改 OpenAPI、Ent 或部署生成物，如出现相关 diff 必须解释或回滚非预期变更。
- [x] 4.3 将本次预期代码、测试和 OpenSpec artifact 变更加到暂存区，再运行 `make lint`。
- [x] 4.4 在暂存本次预期变更后运行 `make verify`，确保最终验证不被未暂存的预期 diff 阻塞。
