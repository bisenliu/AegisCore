## Context

当前用户服务同时存在 `common/validation/` 和 `user-services/internal/validation/`。`common/validation/` 是共享校验核心，配合 `common/http/ginvalidation` 完成 Gin binding、struct tag 校验、错误归一化和统一响应；`user-services/internal/validation/` 是用户服务内的输入清洗、解析和服务特定校验规则集合。

现有服务内校验逻辑集中在一个包文件中，包含用户资料请求、列表分页、用户 ID 解析、登录、改密和刷新 token 输入处理。随着未来 team/toll team 等请求对象和内部输入对象增加，继续使用 `validation` 命名会与共享校验核心产生语义冲突，也不利于按领域扩展。

## Goals / Non-Goals

**Goals:**

- 将用户服务本地校验边界从 `user-services/internal/validation/` 迁移到 `user-services/internal/validators/`。
- 使用 `validators` 作为服务内校验器集合的包名，允许包含 Normalize、Validate、Parse 等函数。
- 将现有 user 与 auth 相关逻辑拆分为 `user.go/user_test.go` 和 `auth.go/auth_test.go`。
- 保持 controller/service/repository 分层不变：controller 绑定请求并调用 validators，service 执行业务编排，repository 访问数据库。
- 保持 `common/validation/` 的共享校验核心职责不变。
- 保持现有 HTTP API 行为和错误语义兼容。

**Non-Goals:**

- 不新增请求字段、路由或响应结构。
- 不修改 Ent schema、数据库 migration 或生成代码。
- 不改变 `common/validation` 的 validator hook、tag rule 或错误归一化行为。
- 不把 repository 查询、唯一性检查、权限判断或会话状态判断下沉到 validators。
- 不引入新的 validator 框架或第三方依赖。

## Decisions

1. 使用 `validators` 而不是 `requestvalidation`

`validators` 是角色命名，表达服务内校验器集合，可以覆盖 API request DTO、内部 input、配置对象或中间状态对象的清洗、校验和转换。`requestvalidation` 虽然更精确描述当前 HTTP 请求场景，但会把包名锁定在 request，对未来非 request 对象的校验语义覆盖不足。

2. 保留 Normalize、Parse 等函数命名

当前服务内规则会修改请求对象，例如 trim、lowercase、Bearer token 清洗、分页默认值计算和 UUID parse。对于会改变输入形态的函数，使用 `NormalizeXxx` 或 `ParseXxx` 比统一改为 `ValidateXxx` 更准确。`validators` 包名用于定义组件角色，不要求包内函数只能执行纯校验。

3. 按领域拆分文件而不是拆分子包

本次迁移后目录结构为：

```text
user-services/internal/validators/
  user.go
  user_test.go
  auth.go
  auth_test.go
```

保持单一 package 可以降低 controller import 和测试维护成本。后续新增 team/toll team 规则时可继续增加 `team.go`、`team_test.go`。只有当单个领域规则显著膨胀时，再考虑二级子包。

4. 保持共享校验能力在 common 中

`common/validation/` 继续负责 validator 初始化、struct tag 校验、字段名解析、错误归一化、自定义 enum rule 和 DTO hook。服务特定规则不得因目录调整上移到 `common/validation/`，除非它们已成为多个服务稳定复用的通用能力。

5. 保持错误响应兼容

validators 仍可返回 `response.ValidationFailedError`、`response.BadRequestError`、`response.TokenInvalidError` 等现有错误类型。controller 继续通过 `response.Fail` 输出统一响应信封，不改变 HTTP 状态码、业务码或公开 message。

## Risks / Trade-offs

- [Risk] `validators` 语义比 `requestvalidation` 更宽，未来可能被滥用为杂物目录。→ Mitigation: 明确边界，validators 不访问 repository、Ent client、外部服务，不执行依赖持久化状态的业务判断。
- [Risk] 文件拆分可能遗漏测试迁移或 import 路径更新。→ Mitigation: 迁移后运行 `go test ./...`，并检查 controller 中不再导入 `user-services/internal/validation`。
- [Risk] 包名变更影响已有 OpenSpec 或文档引用。→ Mitigation: 同步更新相关规格中 `user-services/internal/validation` 的引用为 `user-services/internal/validators`。
- [Risk] 名称从 validation 到 validators 后可能让团队误以为只能写 `ValidateXxx`。→ Mitigation: 保留 `NormalizeXxx`、`ParseXxx` 命名，并在规格中说明 validators 可包含清洗、校验和转换组件。
