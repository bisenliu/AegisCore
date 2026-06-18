# Testing

## Entry Points

| 范围 | 命令 | 说明 |
|---|---|---|
| 全仓库 | `make test` | 从仓库根目录分别在 `common/` 和 `user-service/` 执行测试 |
| 共享模块 | `go test ./...` | 在 `common/` 执行 |
| 用户服务模块 | `go test ./...` | 在 `user-service/` 执行 |

## What To Validate

- Controller：路径参数、绑定校验失败、application 错误映射、成功响应。
- Application command/query：业务编排、端口调用、DTO 映射、应用错误转换。
- Infrastructure adapter：Ent not found、约束冲突、Redis key/TTL/index/session 语义。
- Middleware：OTel Gin span、panic recovery、request logging、CORS、JWT、RBAC。
- Config loader：显式配置、`AEGISCORE_` 覆盖、named Redis/PostgreSQL、非法配置拒绝。
- Runtime/Fx：启动、停止、HTTP timeout、dependency unavailable、graceful shutdown。
- Logging：有效 span context 下携带 `trace_id` / `span_id`，无有效 span context 时不伪造。

## RBAC Regression Scope

RBAC 变更需要覆盖 role command/query、用户角色绑定、角色权限绑定、permission command/query、route diff、Casbin policy loader/enforcer/reload、Gin RBAC middleware 和多实例 policy sync。重点场景包括授权成功、用户无角色拒绝、角色停用拒绝、权限停用拒绝、用户解绑角色拒绝、角色解绑权限拒绝、内置 `super_admin` wildcard 允许访问。

Route diff 测试必须证明它只返回 missing/stale 差异，不创建权限、不修改权限状态、不绑定角色。Casbin 授权使用 `role:<role_uuid>`，不得把 `roles.code` 描述为授权必需字段。

## External Dependencies

普通 `go test ./...` 和 `make test` 不要求 Docker，也不默认启动真实 PostgreSQL/Redis。真实依赖测试通过 `common/testing/containers` 并由环境变量显式启用：

```bash
AEGISCORE_TEST_CONTAINERS=1 go test ./...
```

用户服务 HTTP 全链路 e2e 位于 `user-service/tests/e2e`，默认跳过。启用后使用真实 PostgreSQL/Redis、Atlas SQL migration schema、真实 Gin/Fx assembly、JWT service 和 Redis session adapter：

```bash
cd user-service
AEGISCORE_TEST_E2E=1 go test ./tests/e2e -count=1
```

E2E schema 初始化必须来自 Atlas SQL migration，不得使用 `client.Schema.Create(ctx)`。

## Migration Verification

涉及 Ent schema 或 SQL migration 的变更应验证：

1. 运行 `make user-service-generate`。
2. 运行 `make user-service-migrate-diff name=<name>` 或确认现有 migration 覆盖变化。
3. Review SQL 和 `atlas.sum`。
4. 运行 `make user-service-migrate-validate`。
5. 确认运行时没有自动建表或改 schema。

## Change Verification

每个 change 实现完成后至少执行与改动范围匹配的测试。跨模块变更分别验证 `common` 和 `user-service`。涉及 HTTP API 时验证成功和失败响应均符合 `common/contract/response.Envelope`。涉及 OpenAPI annotation 时运行 `make user-service-openapi-generate` 并检查生成物 drift。
