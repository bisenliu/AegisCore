## Why

当前配置加载会通过 Viper 和后续默认值逻辑为多项运行时配置自动填充默认值，导致主要配置缺失时服务仍可能启动并连接到非预期地址或使用非预期参数。需要移除默认配置行为，让缺失的关键配置在启动阶段直接报错，避免隐式配置掩盖部署问题。

## What Changes

- 移除 `common/config` 中 Viper `SetDefault` 默认配置注册，配置值必须来自 YAML 或 `AEGISCORE_` 环境变量覆盖。
- 调整配置加载和校验逻辑，使主要配置缺失或无效时 `common/config.Load` 返回错误，从而阻止服务启动。
- 保留 YAML + 环境变量覆盖能力，但不再通过代码提供运行时默认值。
- 更新测试，覆盖缺少 app、HTTP、log、Redis、PostgreSQL 等主要配置时加载失败。
- **BREAKING**：依赖代码默认值启动的环境必须补齐显式配置。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-infrastructure`：配置加载要求从“应用默认值后验证”调整为“必须显式提供主要配置，缺失则验证失败并阻止启动”。

## Impact

- 影响代码：`common/config/loader.go`、`common/config/config.go` 及相关配置测试。
- 影响运行时：缺少主要 YAML 配置或环境变量覆盖时，用户服务启动会在配置加载阶段失败。
- 影响配置：`user-services/configs/config.yaml` 必须保持完整声明 app、HTTP、log、Redis、PostgreSQL 等主要配置字段。
- 不影响 HTTP API 路由、响应信封、业务分层或 Ent schema；不需要修改 `user-services/ent/` 生成代码。
