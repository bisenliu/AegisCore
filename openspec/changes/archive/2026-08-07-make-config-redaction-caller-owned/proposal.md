## Why

`common/runtime/config` 当前对 `auth.jwt.secret` 等 user-service 私有配置路径存在认知，破坏了 `common` 只提供业务中立 runtime primitive 的边界。脱敏路径策略应由消费服务显式声明，避免新增服务或 feature secret 时继续修改共享模块。

## What Changes

- 调整 `common/runtime/config` 的脱敏职责：只保留深拷贝、通配路径匹配、按调用方路径脱敏和 YAML 渲染原语。
- 移除 common 中内置的 user-service 私有默认敏感路径，包括 `auth.jwt.secret`。
- 在 user-service 的配置渲染或 CLI 边界集中声明 JWT、Redis、PostgreSQL 及服务私有敏感字段路径，并显式传入共享脱敏原语。
- 补充 nil、空路径、通配 map、多层 slice/map、未知路径、输入不变和 render 防泄漏测试。
- 不改变 YAML merge、strict decode、digest、配置来源加载、Nacos dataId、环境变量名或运行时配置结构。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: 明确配置脱敏原语的业务中立边界、调用方路径所有权和输入不变行为。
- `auth-session-management`: 明确 user-service auth/config 渲染边界拥有 JWT 及相关服务私有敏感路径策略。

## Impact

- 影响代码：`common/runtime/config` 的 redaction/render helper、相关单元测试，`user-service/internal/config` 或 CLI effective settings 渲染调用点。
- 影响安全：防止 common 通过服务私有路径猜测 secret，同时确保 user-service render 输出继续脱敏 JWT、Redis 和 PostgreSQL 凭据。
- 影响共享契约：`RedactSettings` 必须完全由调用方路径定义复用，新服务无需修改 common 即可声明自身敏感字段。
- 不影响外部 API、数据库 schema、OpenAPI、Nacos dataId、环境变量名、配置来源加载流程、raw digest 或 strict decode 行为。
