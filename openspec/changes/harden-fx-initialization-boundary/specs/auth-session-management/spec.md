## ADDED Requirements

### Requirement: auth application constructor 缺失依赖错误
auth application constructor MUST 将可预期的缺失 collaborator 或无效窄 settings 表达为明确 error，MUST NOT 使用 panic 作为正式装配失败路径。constructor MUST 继续保持 application 层边界，不得引入 Fx、HTTP transport DTO 或完整运行时配置依赖。

#### Scenario: token version 本地失效器缺失
- **WHEN** 构造认证 session lifecycle 时缺少 token version local invalidator
- **THEN** constructor MUST 返回明确错误
- **AND** Fx graph MUST 通过标准 error path 拒绝装配
- **AND** constructor MUST NOT panic

#### Scenario: application 边界保持纯净
- **WHEN** auth application constructor 增加错误返回
- **THEN** application 包 MUST NOT 因此导入 `go.uber.org/fx`、Gin、HTTP response helper 或服务完整配置
- **AND** 调用方 MUST 通过普通 Go 错误处理或 Fx constructor error 接收失败
