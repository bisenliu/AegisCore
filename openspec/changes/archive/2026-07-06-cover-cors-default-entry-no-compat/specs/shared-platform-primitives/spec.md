## ADDED Requirements

### Requirement: 共享 CORS 默认入口覆盖

系统 MUST 在 `common/http/middleware` 中为共享 CORS 默认入口 `CORS()` 保持直接单元测试覆盖。测试 MUST 锁定当前默认策略：允许来源为 `*`，允许方法为 `GET,POST,PUT,PATCH,DELETE,OPTIONS`，允许请求头为 `Authorization,Content-Type`；测试 MUST 验证 `CORS()` 与 `CORSWithOptions(defaultCORSOptions)` 的外部响应行为一致，并 MUST NOT 接受旧 origin 反射默认值、旧 header、旧 wildcard+credentials 兼容行为或旧安全兼容开关。

#### Scenario: 默认响应头

- **WHEN** 普通 HTTP 请求经过 `CORS()` middleware
- **THEN** 响应 MUST 包含 `Access-Control-Allow-Origin=*`
- **AND** 响应 MUST 包含 `Access-Control-Allow-Methods=GET,POST,PUT,PATCH,DELETE,OPTIONS`
- **AND** 响应 MUST 包含 `Access-Control-Allow-Headers=Authorization,Content-Type`
- **AND** 默认响应 MUST NOT 包含 `Access-Control-Allow-Credentials`、`Access-Control-Max-Age`、`Access-Control-Expose-Headers` 或 `Vary: Origin`

#### Scenario: 默认预检短路

- **WHEN** `OPTIONS` 预检请求经过 `CORS()` middleware
- **THEN** 系统 MUST 返回 `204 No Content`
- **AND** 业务 handler MUST NOT 被继续调用
- **AND** 响应 MUST 继续包含当前默认 CORS 响应头

#### Scenario: 默认普通请求传递

- **WHEN** 非 `OPTIONS` 普通请求经过 `CORS()` middleware
- **THEN** 系统 MUST 继续调用后续业务 handler
- **AND** 业务 handler 写入的 HTTP status 和 body MUST 保持可见
- **AND** `CORS()` 的相关响应结果 MUST 与 `CORSWithOptions(defaultCORSOptions)` 一致

#### Scenario: CORS 测试断言风格

- **WHEN** 新增或修改 `common/http/middleware` 的 CORS 测试
- **THEN** 常见错误、状态、相等性、布尔条件、集合或字符串断言 MUST 使用语义化 `require` 或允许边界内的 `assert`
- **AND** 测试 MUST NOT 通过机械 `require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf` 替换常见断言
