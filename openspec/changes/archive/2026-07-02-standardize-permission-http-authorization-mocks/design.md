## Context

permission HTTP transport 包中的 command/query controller 测试已经通过 `mock_generate.go` 使用 gomock 生成 mock，但 `authorization_test.go` 仍保留手写 `fakeAuthorizer`。授权中间件本身位于 `user-service/internal/features/permission/transport/http/authorization.go`，通过真实 Gin 路由构造 `Subject`、`Object` 和 `Action`，再调用 permission application authorization port。

本 change 只调整测试与 mock 生成方式，归属 `rbac-access-control` capability。它不改变生产授权流程、Casbin policy、RBAC policy sync、HTTP API、OpenAPI、数据库 schema、部署清单或观测资产。

## Goals / Non-Goals

**Goals:**

- 让 permission HTTP 包内 controller mock 与 authorization middleware mock 都由 gomock 生成。
- 移除 `authorization_test.go` 中的 `fakeAuthorizer`，不保留旧 fake 兼容入口。
- 使用 gomock 精确验证授权中间件传给 authorizer 的 user id、Gin full path 和 HTTP method。
- 保持真实 Gin 路由测试路径，继续覆盖白名单、`OPTIONS`、缺失用户、拒绝、错误和 invalid subject 场景。
- 将 mock 生成物保持在 permission HTTP feature-local 测试包内。

**Non-Goals:**

- 不修改 `Authorize` 中间件生产逻辑、`authorizationAdapter`、Casbin engine 或 permission application authorization service。
- 不调整 command/query controller 测试。
- 不新增中央 mock 仓库，不把 mock 放入 `common`、`internal/shared` 或跨 feature 目录。
- 不变更 HTTP API、OpenAPI、Ent schema、Atlas migration、部署资产或观测配置。

## Decisions

1. 在 `mock_generate.go` 中增加 `authorization.Authorizer` 的 go:generate 入口。

   理由：该文件已经集中管理 permission HTTP 包内测试 mock 的生成入口，继续使用它可以保持同包 mock 风格一致，并让 `make user-service-generate` 统一维护生成物。

   备选方案：继续手写 `fakeAuthorizer`。拒绝原因是手写 fake 与既有 gomock 风格不一致，且容易遗漏未调用断言和参数匹配。

2. 生成物放在 permission HTTP 包本地测试文件中。

   理由：`authorization_test.go` 只在 `user-service/internal/features/permission/transport/http` 内消费该 mock，feature-local 生成物符合仓库边界，也避免形成跨 feature 或中央 mock 依赖。

   备选方案：建立共享 mock 目录。拒绝原因是当前只有本包测试消费，中央 mock 仓库会扩大边界并增加维护成本。

3. 测试继续通过真实 Gin engine 和 route template 执行中间件。

   理由：本次要验证的是 middleware 与 Gin `FullPath()`、HTTP method、认证上下文之间的组合行为，直接调用 helper 会降低覆盖价值。

   备选方案：仅单测 `authorizationAdapter` 或 `resolveAuthorizationRequest`。拒绝原因是无法完整覆盖白名单、`OPTIONS` bypass 和真实路由模板解析路径。

4. 白名单和 `OPTIONS` 场景使用 gomock 的“无期望调用”语义验证 bypass。

   理由：不设置 `Enforce` 期望即可让 gomock 在发生额外调用时失败，比手写 `calls == 0` 更贴近同包 controller mock 的断言风格。

   备选方案：保留显式计数器断言。拒绝原因是这会继续依赖手写 fake。

## Risks / Trade-offs

- [Risk] 生成的 mock 文件名与现有 `mock_test.go` 或 `mock_query_test.go` 冲突。→ 使用独立 destination，并在 `make user-service-generate` 后检查 diff。
- [Risk] gomock 参数匹配过宽，导致 user id、full path 或 method 回归未被发现。→ 对成功、拒绝、错误和 invalid subject 场景使用精确参数匹配。
- [Risk] 白名单或 `OPTIONS` bypass 测试误设期望，无法证明 authorizer 未调用。→ bypass 子测试不设置 `Enforce` 期望，依赖 gomock 自动失败机制。
- [Risk] mock 生成命令引入跨层依赖。→ 仅引用 permission application authorization port，不放入 `common`、`shared`、`integration` 或其他 feature。

## Migration Plan

1. 更新 `mock_generate.go`，增加 `authorization.Authorizer` mock 生成入口。
2. 执行 `make user-service-generate` 生成或刷新 feature-local mock 文件。
3. 改造 `authorization_test.go` 使用 gomock，并删除 `fakeAuthorizer`。
4. 执行 `cd user-service && go test ./internal/features/permission/transport/http`、`make user-service-architecture-lint` 和生成 drift 检查。

回滚方式：还原本 change 涉及的 `mock_generate.go`、生成 mock 文件和 `authorization_test.go` 改动即可；由于不涉及生产逻辑、schema 或部署资产，不需要数据迁移或运行时回滚步骤。

## Open Questions

- 无。
