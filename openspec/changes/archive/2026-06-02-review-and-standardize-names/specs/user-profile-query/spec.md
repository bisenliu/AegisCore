## ADDED Requirements

### Requirement: User profile query naming cleanup preserves layered behavior
用户资料查询相关命名标准化 SHALL 保持 controller/service/repository 分层职责不变，并不得改变 `GET /api/v1/users/:id` 的请求解析、业务编排、数据访问、错误映射或响应内容。

#### Scenario: Internal user query symbols are renamed
- **WHEN** 实现重命名用户查询相关内部函数、方法、参数、mapper 或类型
- **THEN** controller 仍 MUST 只处理 HTTP 解析和响应输出，service 仍 MUST 负责编排，repository 仍 MUST 负责数据库访问

#### Scenario: User query API remains compatible
- **WHEN** 命名标准化完成
- **THEN** `GET /api/v1/users/:id` 的路径、响应 envelope、用户响应 JSON 字段和错误语义 MUST 保持不变
