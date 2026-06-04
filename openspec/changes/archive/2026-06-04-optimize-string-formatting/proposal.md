## Why

当前代码中存在少量将 `fmt.Sprint`、`fmt.Sprintf` 用于简单类型转换或字符串拼接的场景，其中 `user-services/internal/domain/user_status.go` 的枚举允许值生成会引入不必要的通用格式化开销。该变更用于在不降低可读性的前提下消除可确认的低效实现，并建立一次项目范围内的判断记录。

## What Changes

- 检查并优化 `shared-enum-contracts` 相关的用户状态枚举字符串转换实现，优先替换不必要的 `fmt.Sprint`。
- 在整个 Go workspace 范围内搜索仅用于简单类型转换或拼接的 `fmt.Sprint`、`fmt.Sprintf` 等用法。
- 对可读性不下降且性能收益明确的场景改用更直接的实现，例如常量字符串、`strconv` 或普通字符串拼接。
- 对格式化语义明确、调试/错误消息可读性更重要、或替换后收益不明显的场景保持不变，并在结果中说明判断依据。
- 不改变 HTTP API、错误码、配置、数据库模型或业务行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-enum-contracts`: 优化枚举允许值字符串化实现，但不改变枚举值、校验语义或对外契约。

## Impact

- 主要影响代码位置：`user-services/internal/domain/user_status.go`。
- 搜索范围包含 `common/` 与 `user-services/` 两个 Go 模块中的字符串格式化和拼接用法。
- 外部可观察行为保持兼容：API 响应、错误码、配置格式、数据库 schema 和迁移均不变。
- 依赖不变，不新增第三方库。
