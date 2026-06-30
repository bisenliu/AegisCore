## 1. auth port 拆分

- [x] 1.1 在 `user-service/internal/features/auth/application/ports.go` 拆分 `AuthSessionStore`，新增 token version cache 与 refresh session store 最小接口，并保留 `UserTokenVersionStore` 的持久化职责。
- [x] 1.2 更新 `application/validators`，使 token version validator 只依赖 token version 持久化仓储和 token version cache，不依赖 refresh session CRUD 或批量删除接口。
- [x] 1.3 更新 `application/sessions`，使 session 创建、校验、轮换、删除和撤销编排依赖拆分后的最小 port。
- [x] 1.4 更新 Redis session store concrete type 的接口实现断言、构造和测试桩，使同一 adapter 实现拆分后的 Redis cache/store port，但不新增业务编排逻辑。
- [x] 1.5 更新 Fx provider、auth component 构造函数和所有测试 helper，消除旧 `AuthSessionStore` 宽接口依赖残留。

## 2. 全部会话撤销语义

- [x] 2.1 调整 `RevokeAllUserSessions` 和 `RevokeUserSessionsAtVersion` 的结果或错误处理，使 PostgreSQL token version 递增成功与 Redis 投影/refresh session 删除失败的语义可区分。
- [x] 2.2 保持 token version 递增成功后旧 access token 失效的主事实不变，并确保 Redis token version 投影写入失败时尝试删除投影以回源 PostgreSQL。
- [x] 2.3 保持全部会话后台物理清理不承担安全失效主事实，确认 purge workerpool 只负责批量删除已摘除的 Redis refresh session key。
- [x] 2.4 更新 logout all 和 change password 相关测试，覆盖投影成功、cache 写入失败、cache 删除失败、session 批量删除失败和主事实成功但投影失败的观测语义。

## 3. 验证和规格收尾

- [x] 3.1 运行 `rg "AuthSessionStore" user-service/internal/features/auth`，确认旧宽接口无残留。
- [x] 3.2 运行 `rg "string\\(authapplication\\.Metrics" user-service/internal/features/auth`，确认本 change 未引入 auth metrics 强类型转换。
- [x] 3.3 运行 auth application、Redis session store 和 auth metrics 相关 package 测试，至少覆盖 `user-service/internal/features/auth/application/...` 与 `user-service/internal/features/auth/infrastructure/redis`。
- [x] 3.4 运行 `make user-service-architecture-lint`，确认 auth feature 分层、OpenSpec 文档和架构边界通过。
- [x] 3.5 运行 `make lint` 和 `make verify`，确认仓库级验证通过。
- [x] 3.6 运行 `git diff --exit-code -- user-service/docs/openapi.go user-service/docs/openapi.json user-service/docs/openapi.yaml user-service/ent`，确认本 change 未产生 OpenAPI 或 Ent 生成物漂移。
