## ADDED Requirements

### Requirement: OpenAPI 转换工具测试断言
OpenAPI 转换和生成链路相关工具测试 MUST 使用语义化断言验证转换错误、OpenAPI JSON/YAML 内容、生成文件路径和生成物存在性。测试断言迁移 MUST NOT 改变 OpenAPI 文档路由、OpenAPI 生成物、Swagger/OpenAPI 转换输出契约或服务专属生成参数。

#### Scenario: 验证 OpenAPI 转换输出
- **WHEN** 工具测试验证 Swagger 2 到 OpenAPI 3 的转换结果、JSON/YAML 输出或生成文件内容
- **THEN** 测试 MUST 优先使用 `require.JSONEq`、`require.Contains`、`require.ElementsMatch`、`require.Len`、`require.Regexp` 或等价语义化断言
- **AND** 测试 MUST NOT 使用手写字符串拼接失败消息或布尔包装替代已有专属断言

#### Scenario: 保持 OpenAPI 生成契约不变
- **WHEN** 迁移 OpenAPI 转换工具测试断言
- **THEN** 系统 MUST NOT 修改 `make user-service-openapi-generate` 的输出文件集合
- **AND** 系统 MUST NOT 修改 OpenAPI UI/JSON 路由、认证方案、扫描范围、CLI flag 或服务脚本传入的生成参数
