## Why

`request-validation` 当前把纯结构体校验、Gin 请求绑定、失败响应和日志出口集中在 `common/validation` 包中，导致所有校验消费者都被动依赖 Gin HTTP 边界。现在拆分适配层可以在不改变现有 API 响应语义的前提下，降低共享校验核心与 Gin transport 的耦合。

## What Changes

- 新增 `common/ginvalidation` 作为 Gin 请求绑定与 `BindOrAbort` HTTP 适配层。
- 保留 `common/validation` 作为共享 validator 初始化、结构体校验、字段明细、错误归一化和自定义 enum 规则的核心包。
- 将 JSON、strict JSON、URI、query、form binder 以及 Gin 失败响应/日志处理迁移到 `common/ginvalidation`。
- 更新用户服务 controller 使用新的 Gin validation adapter，保持现有请求校验、失败信封、错误码和日志行为兼容。
- 不改变现有 HTTP API 路由、响应信封字段、错误码、配置或数据模型。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `request-validation`: 调整共享请求校验能力的实现边界，明确纯校验核心与 Gin HTTP 适配职责分离，同时保持现有请求绑定、校验失败响应和日志语义。

## Impact

- 受影响代码：`common/validation/`、新增 `common/ginvalidation/`、`user-services/internal/controller/` 及相关测试。
- 受影响能力：`request-validation`，并通过现有依赖间接受 `user-profile-create`、`user-profile-query`、`user-list-query`、`user-session-control` 等 controller 请求校验路径覆盖。
- 外部兼容性：HTTP 状态码、`common/response.Envelope`、业务错误码、字段级校验明细和日志字段保持兼容。
- 依赖影响：Gin 依赖从 `common/validation` 核心包移到 `common/ginvalidation` 适配包；项目仍继续使用 Gin HTTP 栈。
