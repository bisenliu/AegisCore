## ADDED Requirements

### Requirement: 仓库级 OpenAPI 转换工具测试验证

系统 MUST 为 `tools/openapi-convert` 提供默认可执行的 Go 测试，覆盖 CLI 参数解析、错误路径和文件生成结果。根 `make test` 和 `make verify` MUST 执行该工具模块测试，工具模块测试失败时完整验证 MUST 失败。

#### Scenario: 根测试覆盖 OpenAPI 转换工具
- **WHEN** 协作者执行 `make test`
- **THEN** 系统 MUST 执行 `tools/openapi-convert` 模块的 Go 测试
- **AND** 系统 MUST 同时保持 `common` 和 `user-service` 模块测试执行

#### Scenario: 完整验证覆盖 OpenAPI 转换工具
- **WHEN** 协作者执行 `make verify`
- **THEN** 系统 MUST 通过测试阶段执行 `tools/openapi-convert` 模块测试
- **AND** 工具模块测试失败 MUST 阻止 `make verify` 成功完成

#### Scenario: CLI 参数错误回归测试
- **WHEN** `tools/openapi-convert` 测试覆盖缺少必填 `input`、`json`、`yaml` 或 `go` 输出路径的调用
- **THEN** 测试 MUST 断言 CLI 返回失败结果并输出明确错误

#### Scenario: root path 参数约束回归测试
- **WHEN** `tools/openapi-convert` 调用设置 `root-path` 但未设置 `root-server`
- **THEN** CLI MUST 返回失败结果
- **AND** 测试 MUST 断言该约束错误被保留

#### Scenario: 文件生成回归测试
- **WHEN** `tools/openapi-convert` 使用合法 Swagger 2 输入和输出路径执行
- **THEN** 测试 MUST 断言 JSON、YAML 和 Go embed 输出文件被创建
- **AND** 测试 MUST 断言生成内容包含 OpenAPI 版本、路径或 Go package 等关键结构

#### Scenario: 输入输出错误回归测试
- **WHEN** `tools/openapi-convert` 收到不存在的输入文件或不可写输出目标
- **THEN** CLI MUST 返回失败结果
- **AND** 测试 MUST 断言错误信息能定位输入转换或输出写入阶段
