## ADDED Requirements

### Requirement: Load system timezone configuration

系统 MUST 从 YAML 配置和 `AEGISCORE_` 环境变量覆盖中加载 `system.timezone` 到共享配置对象。配置加载器 MUST 只负责读取、覆盖和反序列化该字段，不得因为该字段缺失、为空或取值无法加载为 IANA 时区而在 `common/config.Load` 阶段返回校验错误。

#### Scenario: Load timezone from YAML
- **Given** YAML 配置包含 `system.timezone: Asia/Shanghai`
- **When** `common/config.Load` 被调用
- **Then** 系统 MUST 将该值反序列化到 `config.Config` 的系统配置中

#### Scenario: Override timezone with environment variable
- **Given** YAML 配置包含 `system.timezone: Asia/Shanghai`
- **Given** 环境变量提供 `AEGISCORE_SYSTEM_TIMEZONE=UTC`
- **When** `common/config.Load` 被调用
- **Then** 系统 MUST 将系统时区配置加载为 `UTC`

#### Scenario: Missing timezone is not rejected by config loader
- **Given** YAML 和环境变量未显式提供 `system.timezone`
- **When** `common/config.Load` 被调用
- **Then** 配置加载 MUST 成功反序列化配置对象
- **Then** 配置加载器 MUST NOT 因系统时区为空而返回校验错误

### Requirement: Provide shared timezone initialization module

系统 MUST 在 `common` 模块提供可复用的 timezone Fx module。该 module MUST 基于共享配置初始化进程本地时区，默认使用 `Asia/Shanghai`，成功时设置 `time.Local` 并同步 `TZ` 环境变量。无效时区 MUST 返回启动错误并保留底层加载错误上下文。初始化 MUST 在进程内只执行一次。

#### Scenario: Initialize configured timezone
- **Given** 配置中 `system.timezone` 为 `UTC`
- **When** 服务引入共享 timezone module 并启动 Fx app
- **Then** 系统 MUST 加载 `UTC` 时区
- **Then** 系统 MUST 将 `time.Local` 设置为该时区
- **Then** 系统 MUST 将 `TZ` 环境变量设置为 `UTC`

#### Scenario: Initialize default timezone
- **Given** 配置中未提供 `system.timezone`
- **When** 服务引入共享 timezone module 并启动 Fx app
- **Then** 系统 MUST 使用默认时区 `Asia/Shanghai`
- **Then** 系统 MUST 将 `time.Local` 设置为该时区
- **Then** 系统 MUST 将 `TZ` 环境变量设置为 `Asia/Shanghai`

#### Scenario: Invalid timezone fails startup
- **Given** 配置中 `system.timezone` 为无法被 `time.LoadLocation` 加载的值
- **When** 服务引入共享 timezone module 并启动 Fx app
- **Then** 系统 MUST 返回启动错误
- **Then** 错误 MUST 包含失败的时区值或底层加载错误上下文
- **Then** 服务 MUST NOT 以不确定时区继续启动

#### Scenario: Timezone initialization is process-global once
- **Given** 进程内已经成功执行过共享 timezone 初始化
- **When** 后续 Fx app 或依赖图再次触发 timezone 初始化
- **Then** 系统 MUST NOT 重复修改 `time.Local` 或 `TZ`
- **Then** 系统 MUST 保持第一次成功初始化的时区设置
