## Context

`common/contract/response` 是 `api-response-contract` 的代码边界，包含成功响应信封、失败响应信封、应用错误构造、分页模型、标准消息和语义响应 helper。现有 `failure.go` 文件名容易让维护者理解为只包含失败信封或失败模型，但文件实际包含面向调用方的语义便利函数，例如 `BadRequest`、`ValidationFailed`、`Unauthenticated`、`Forbidden`、`Conflict` 和 `NotFound`。

本变更是 common contract 包内的文件组织改名，不改变 package、导出符号或任何 HTTP 可观察行为。

## Goals / Non-Goals

**Goals:**

- 将 `common/contract/response/failure.go` 重命名为 `common/contract/response/helpers.go`。
- 使用 `helpers.go` 表达该文件承载语义响应 convenience utilities 的职责。
- 保持 `common/contract/response` 包名、导出 API、统一响应信封、HTTP status、业务错误码、错误映射和公开 message 不变。
- 保持共享响应能力在 `common/contract/response` 内，不影响 controller/service/repository 分层。

**Non-Goals:**

- 不新增、删除或重命名 `response.BadRequest`、`response.ValidationFailed`、`response.Unauthenticated`、`response.Forbidden`、`response.Conflict`、`response.NotFound` 等导出函数。
- 不修改 `Envelope`、`AppError`、分页模型、错误码常量、message 常量或 JSON 字段。
- 不调整 HTTP 路由、中间件、controller、service、repository 或 Swagger 行为。
- 不涉及 Ent schema、Atlas migration、Redis/PostgreSQL 配置、Fx provider 或 Go toolchain。

## Decisions

1. 使用 `helpers.go` 而不是 `semantic.go`

`helpers.go` 与 Go 项目中承载包内 convenience functions 的常见命名一致，能够覆盖成功、失败、校验和认证等语义 helper。`semantic.go` 表达更抽象，不如 `helpers.go` 直接描述维护者查找便利函数时的文件职责。

2. 仅重命名源文件，不拆分 helper

当前变更意图是提高文件命名可读性。将各类 helper 拆分到 `auth.go`、`validation.go` 或 `errors.go` 会扩大改动范围，并可能与现有 `error.go`、`response.go` 等文件职责产生新的边界讨论。本次保留函数集合，只调整文件名。

3. 保持 Go package 和导出 API 不变

Go 编译和调用方 import path 以 package 为边界，文件名不属于外部 API。保持 `package response` 和所有导出符号不变，可以确保用户服务 controller、中间件、测试和 Swagger-adjacent 代码无需改变调用方式。

4. 不引入生成、迁移或运行时步骤

该变更不触碰 Ent schema、数据库 schema、配置加载、Redis/PostgreSQL 连接或 Fx 生命周期，因此不需要 `go generate`、Atlas migration、配置兼容策略或部署迁移步骤。

## Risks / Trade-offs

- [Risk] 文档、脚本或测试中存在对 `failure.go` 文件名的直接引用。-> Mitigation: apply 阶段搜索 `failure.go` 并同步更新仅文件名相关引用。
- [Risk] 维护者误以为可以借重命名调整 helper 行为。-> Mitigation: 任务和规格明确限制为文件名变更，验证现有测试并保持响应契约不变。
- [Risk] `helpers.go` 命名较宽，未来可能吸收过多无关逻辑。-> Mitigation: 限定其职责为 `common/contract/response` 内的语义响应 convenience utilities，非响应契约逻辑不得进入该文件。
