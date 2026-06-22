## ADDED Requirements

### Requirement: localcache 包内结构保持稳定契约
系统 MUST 允许 `common/runtime/localcache` 按错误变量、公开类型和核心实现拆分包内文件，同时保持 `package localcache`、导出 API、错误变量和运行时行为不变。

#### Scenario: 拆分包内文件
- **WHEN** `common/runtime/localcache` 将错误变量、公开类型和 `Cache` 实现拆分到不同源码文件
- **THEN** 系统 MUST 保持原有导出符号、错误语义、Ristretto 配置、TTL、singleflight、stats 和 `Close` 行为不变
