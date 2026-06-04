## ADDED Requirements

### Requirement: Avoid unnecessary enum formatting overhead
系统 SHALL 在枚举允许值、枚举契约值或固定常量字符串化实现中避免使用不必要的通用格式化函数，但 MUST 保持既有契约值和可读性。

#### Scenario: Replace fixed enum value formatting
- **WHEN** 枚举字符串输出只包含编译期已知且外部契约稳定的数字或字符串常量
- **THEN** 实现 MUST 使用字符串字面量、`strconv` 或普通字符串拼接等更直接的方式替代不必要的 `fmt.Sprint` 或 `fmt.Sprintf`
- **THEN** 输出的枚举允许值 MUST 与变更前保持一致

#### Scenario: Keep readable semantic formatting
- **WHEN** `fmt.Sprintf` 或类似格式化函数用于错误消息、日志、复合模板、格式控制或调试输出
- **THEN** 实现 MUST 仅在替换后可读性不下降且输出完全一致时修改
- **THEN** 对未修改场景 MUST 在实现结果中说明保留依据

#### Scenario: Preserve enum contract compatibility
- **WHEN** 优化枚举字符串化或常量拼接实现
- **THEN** 枚举数字值、JSON/text 反序列化、API 响应、错误码、配置 key 和数据库 schema MUST 保持不变
