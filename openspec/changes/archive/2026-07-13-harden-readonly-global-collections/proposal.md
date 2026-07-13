## Why

当前非生成代码中存在若干 package-level map、slice 和默认 struct 用作只读集合或默认配置，但 Go 语言无法阻止同包未来代码误写这些共享底层状态。该变更通过更难被修改的表达方式加固这些只读数据，降低配置校验、metrics label、HTTP method allowlist 和默认 CORS 策略被无意改变的风险。

## What Changes

- 将配置校验允许值、HTTP method allowlist、弱密钥 denylist、metrics label 集合等只读 map/slice 评估并迁移为 `switch`、局部构造、私有查询 helper、数组常量式表达或返回副本的函数。
- 收紧 `common/http/middleware` 默认 CORS 配置，确保调用方无法通过共享 slice 底层数组修改默认值。
- 对保留的 package-level 变量逐项确认其不属于本次只读集合风险，或具备明确保留理由，例如 sentinel error、regexp 编译结果、Fx Module、`sync.Pool`、atomic counter 等。
- 保持现有运行时行为、配置校验允许值、错误消息、metrics label 名称、CORS 默认策略和 RBAC HTTP method 校验语义不变。
- 不修改 `user-service/ent/` 生成代码，不引入复杂抽象、反射或为了形式不可变性而扩大公开 API。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: 加固 `common` 中配置校验、HTTP middleware、validation 等共享 primitive 的只读集合和默认配置表达，保持对外契约不变。
- `runtime-observability`: 加固 runtime metrics label 集合和 scheduler/HTTP metrics 相关只读数据表达，保持 Prometheus metric family、label key/value 和数值语义不变。
- `rbac-access-control`: 加固 permission domain 中 HTTP method allowlist 表达，保持权限目录输入校验和 RBAC 授权 method 语义不变。

## Impact

- 主要影响代码：`common/runtime/config/validation.go`、`common/runtime/observability/metrics/labels.go`、`common/runtime/observability/metrics/scheduler.go`、`common/http/middleware/metrics.go`、`common/http/middleware/cors.go`、`common/validation/types.go`、`user-service/internal/features/permission/domain/method.go`。
- API 影响：无公开 HTTP API、OpenAPI、数据库 schema、Atlas migration、部署资产或配置文件格式变更。
- 行为影响：用户可见错误语义、配置允许值、弱密钥 denylist、metrics label、CORS 默认响应头和权限 method allowlist 必须保持不变。
- 验证影响：需要在 `common` 模块运行 `go test ./runtime/config ./runtime/observability/metrics ./http/middleware ./validation`，并在 `user-service` 模块运行权限 HTTP method 校验相关测试。
