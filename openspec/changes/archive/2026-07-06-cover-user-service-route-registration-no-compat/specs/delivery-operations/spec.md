## ADDED Requirements

### Requirement: 聚合路由测试断言与覆盖率验收
系统 MUST 为 user-service 聚合路由注册补充符合交付断言规范的 Go 测试，并通过覆盖率验证确保 `RegisterUserServiceHTTPRoutes` 和 `registerV1Routes` 均被执行。

#### Scenario: 语义化断言
- **WHEN** 新增或修改 `user-service/internal/router` 路由注册测试
- **THEN** 测试 MUST 优先使用 `require` 的语义化断言表达错误、集合、字符串、长度和包含关系
- **AND** 存在 `Len`、`Contains`、`ElementsMatch`、`ErrorContains`、`Regexp` 或等价更具体断言时，测试 MUST NOT 使用 `True` 或 `False` 包装布尔表达式
- **AND** 只有多个互相独立的 route 条目需要一次性收集失败且后续检查不依赖前置结果时，测试 MAY 使用 `assert`

#### Scenario: router 覆盖率验收
- **WHEN** 本 change 实施完成
- **THEN** `go test -cover ./user-service/internal/router` MUST 通过
- **AND** `go tool cover -func` MUST 显示 `RegisterUserServiceHTTPRoutes` 和 `registerV1Routes` 均有覆盖
- **AND** `openspec validate cover-user-service-route-registration-no-compat` MUST 通过
