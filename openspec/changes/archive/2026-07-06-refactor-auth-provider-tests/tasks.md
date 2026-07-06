## 1. 基线与生成入口

- [x] 1.1 记录 `session_store_test.go`、`service_test.go`、`routes_test.go` 和 `gin_test.go` 当前 `Test` 函数清单，保存为迁移核对依据。
- [x] 1.2 检查 auth、provider、cmd 和相关包内复杂 fake/stub/spy，标记必须迁移到 `mockgen` 的外部 collaborator 替身。
- [x] 1.3 为缺少生成 mock 的复杂 collaborator 增加或扩展包内 `mock_generate.go`，确保入口使用可复现的 `go tool mockgen` 或仓库既有等价入口。
- [x] 1.4 执行仓库约定生成命令，更新 mock 生成物和 metrics no-op 生成物，并检查生成物 diff 只包含本 change 预期内容。

## 2. auth Redis session store 测试拆分

- [x] 2.1 将 token version cache 相关测试从 `session_store_test.go` 拆分到按主题命名的 `_test.go` 文件。
- [x] 2.2 将 token version validator 与 localcache 回源相关测试拆分到独立 `_test.go` 文件，保留真实 `localcache` 语义验证。
- [x] 2.3 将 refresh session 创建、查询、删除和上限裁剪测试拆分到独立 `_test.go` 文件。
- [x] 2.4 将 refresh session rotation 测试拆分到独立 `_test.go` 文件。
- [x] 2.5 将全量 session 删除、批量 purge、purge pool 和 Fx lifecycle 测试拆分到独立 `_test.go` 文件。
- [x] 2.6 将 Redis key schema、app name prefix、legacy key 忽略和 TTL 相关测试拆分到独立 `_test.go` 文件。
- [x] 2.7 将共享 miniredis、SessionStore 构造、wait helper 和允许保留的轻量 pool helper 移到 `session_store_test_helpers_test.go` 或等价主题 helper 文件。
- [x] 2.8 删除被拆空的旧跨主题 `session_store_test.go`，并用迁移前后 `Test` 函数清单确认无遗漏。

## 3. auth command use case 测试拆分

- [x] 3.1 将 login 成功、失败、TTL、session 上限和 metrics reason 测试拆分到 `login_test.go` 或等价主题文件。
- [x] 3.2 将 change-password、token version 提升、投影失败和 token subject 校验测试拆分到 `change_password_test.go` 或等价主题文件。
- [x] 3.3 将 refresh、rotation、签发失败、session 创建失败、版本不匹配和 metrics 测试拆分到 `refresh_test.go` 或等价主题文件。
- [x] 3.4 将 logout current 与 logout all 测试拆分到 `logout_test.go` 或等价主题文件。
- [x] 3.5 将共享 use case fixture、领域值构造、token claims 和密码 helper 移到 `service_test_helpers_test.go`。
- [x] 3.6 确认 command 包测试继续使用现有 `mockgen` 生成物表达 collaborator expectation，不新增复杂手写 fake/stub。
- [x] 3.7 删除被拆空的旧跨 use case `service_test.go`，并用迁移前后 `Test` 函数清单确认无遗漏。

## 4. provider routes 与 Gin engine 测试拆分

- [x] 4.1 将 route auth middleware、公有路由、受保护路由、RBAC 拒绝和认证 envelope 测试拆分到 routes 主题文件。
- [x] 4.2 将 metrics path conflict 测试拆分到独立 routes 主题文件。
- [x] 4.3 将 provider routes 共享配置、validator、token 签发、response 断言和允许保留的轻量 stats source helper 移到 routes helper 文件。
- [x] 4.4 将 Gin tracing、traceparent 传播和 span status 测试拆分到 tracing 主题文件。
- [x] 4.5 将 request ID 透传、生成和日志字段测试拆分到 request ID 主题文件。
- [x] 4.6 将 HTTP server metrics、runtime endpoint skip 和 unmatched route metrics 测试拆分到 metrics 主题文件。
- [x] 4.7 将 panic recovery 与 panic span event 测试拆分到 panic 主题文件。
- [x] 4.8 删除被拆空的旧 `routes_test.go` 和 `gin_test.go` 跨主题内容，并用迁移前后 `Test` 函数清单确认无遗漏。

## 5. 复杂替身与 no-op 生成约定收敛

- [x] 5.1 将需要断言调用契约的 provider、cmd、auth fx 或 Redis session store 复杂 fake/stub/spy 替换为包内生成 mock 或更窄的主题 helper。
- [x] 5.2 保留真实 Redis/miniredis、真实 `localcache`、领域值构造和无行为分支 stats source helper，并确认它们不替代外部 port 调用断言。
- [x] 5.3 确认 `auth/application/metrics.go` 与 `permission/application/metrics.go` 继续使用统一 `nopgen` 生成约定，且 `common/runtime/observability/metrics` 不新增 user-service 业务指标方法。
- [x] 5.4 重新执行生成命令并检查 `git diff --exit-code` 或等价 drift 检查，确保生成物同步。

## 6. 验证与收尾

- [x] 6.1 运行 `go test ./user-service/internal/features/auth/infrastructure/redis ./user-service/internal/features/auth/application/command ./user-service/internal/providers` 并修复失败。
- [x] 6.2 运行 `make test` 并修复失败。
- [x] 6.3 运行 `make user-service-architecture-lint`，确认 OpenSpec 中文约束和架构边界通过。
- [x] 6.4 检查 `git diff`，确认无业务 API、OpenAPI、Ent schema、Atlas migration、部署资产或观测资产的非预期变更。
- [x] 6.5 将本 change 的预期代码、生成物、规格和文档变更加到暂存区。
- [x] 6.6 在暂存预期变更后运行 `make lint` 并修复失败。
- [x] 6.7 在暂存预期变更后运行 `make verify`，确保最终 drift 检查只暴露未纳入暂存区的意外变更且命令通过。
