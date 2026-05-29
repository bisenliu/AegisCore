## Why

当前 `user-services/internal/controller/user_controller.go` 在 controller 内部直接创建 `validator.Validate` 并手写参数解析错误处理，后续新增 JSON、query、URI 请求 DTO 时会重复实现绑定、校验、错误消息和默认值逻辑。外部 `/Users/liubisen/Desktop/sander/Project/my/go-micro-scaffold/common/pkg/validation` 已覆盖这些问题的一部分，但其实现与 AegisCore 的模块路径、响应契约、日志约定和 Fx 边界不兼容，直接迁移风险较高。

本变更将提炼外部模块中有价值的请求校验能力，优化其健壮性与 Go 实践，并作为 `common` 共享能力集成到当前服务，以统一请求校验规范、降低 controller 重复代码，并为未来新增写接口做好基础设施准备。

## What Changes

- 新增共享请求校验能力，提供统一 `*validator.Validate` 初始化、Gin URI/query/JSON/form binding 辅助函数、DTO 默认值钩子、自定义业务校验错误和枚举校验规则。
- 将外部 validation 模块的设计调整为适配 AegisCore：使用 `github.com/aegiscore/common/...` 模块路径、`common/response.Envelope` 失败响应、`common/logger` trace-id 日志，以及 Fx provider。
- 优化原实现中的潜在 Bug：避免 `ValidateEnum` 类型断言 panic、避免只注册中文翻译却声明英文支持、避免全局 Gin validator 多次注册导致副作用、避免 `removeTopStruct` 依赖翻译字符串分隔符、避免 JSON tag `"_"` 判断错误。
- 在用户查询接口中可选择复用共享 URI 校验能力，同时保持 `GET /api/v1/users/:id` 的外部响应语义不变。
- 非目标：不引入认证、权限、业务写接口、数据库 schema 变更或新的错误码体系。

## Capabilities

### New Capabilities
- `request-validation`: 覆盖共享请求绑定、结构体校验、字段名解析、自定义规则、默认值钩子和校验失败响应映射。

### Modified Capabilities
- `user-profile-query`: 将用户 ID URI 参数校验从 controller 私有 validator 迁移为共享请求校验能力，同时保持非数字或非正 ID 均返回 HTTP 400 和 `invalid user id` 的既有契约。
- `shared-infrastructure`: 通过 Fx 提供共享 validator 运行时依赖，允许服务模块注入复用，不改变配置加载器的 required/range 校验边界。

## Impact

- 影响代码：新增 `common/validation/`；调整 `common/infrastructure/module.go` 或新增 validation Fx module；可更新 `user-services/internal/controller/user_controller.go` 使用共享校验器。
- API 兼容性：`GET /api/v1/users/:id` 的 HTTP 状态码、错误码和错误消息保持兼容；未来新增接口可获得更细粒度字段错误消息，但仍必须通过 `common/response.Envelope` 输出。
- 依赖影响：`common` 需要显式依赖 `github.com/go-playground/validator/v10`，如启用本地化翻译还需要显式依赖 `github.com/go-playground/locales` 和 `github.com/go-playground/universal-translator`。
- 配置影响：本变更不要求 `common/config.Load` 校验配置字段；如后续暴露 locale 配置，应由 validation provider 读取默认值并在初始化阶段校验自身配置。
- 数据影响：不涉及 Ent schema、Atlas migration、Redis 或 PostgreSQL 数据结构变更。
