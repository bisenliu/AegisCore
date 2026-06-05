## Why

`user-services/internal/bootstrap/app.go` 中的 `UserServiceModule` 实际装配了完整进程级 HTTP 运行时依赖，但名称容易被理解为单一用户业务 service 层模块。该命名与 `service.NewUserService` 的概念层级过近，会增加维护者误判 Fx 模块职责边界的风险。

## What Changes

- 将用户服务 Fx 组装模块从业务 service 层语义调整为进程级运行时语义。
- 保持现有 HTTP 启动、路由注册、中间件、PostgreSQL、Redis、JWT、Ent、repository、service、controller 和 graceful shutdown 行为不变。
- 更新直接引用该 Fx 模块的代码命名，使模块职责与 `http-service-runtime` capability 保持一致。
- 不引入新的 API、错误码、配置项、数据库模型或外部行为变化。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `http-service-runtime`: 要求用户服务 Fx 运行时组装模块使用进程级运行时语义命名，避免与业务 service 层模块混淆。

## Impact

- 影响代码：`user-services/internal/bootstrap/app.go` 及任何直接引用原 Fx 模块符号的位置。
- API 兼容性：无外部 HTTP API、响应信封、错误码或请求/响应结构变化。
- 配置兼容性：无 YAML 或 `AEGISCORE_` 环境变量变化。
- 数据兼容性：无 Ent schema、Atlas migration 或持久化数据变化。
