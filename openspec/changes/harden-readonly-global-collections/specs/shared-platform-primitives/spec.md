## ADDED Requirements

### Requirement: 共享只读集合不得暴露共享可写状态
`common` 中用于配置校验、HTTP middleware 默认策略和 validation tag 解析的只读集合或默认 struct MUST 使用不暴露共享可写底层状态的表达方式。实现 MUST 保持配置允许值、弱密钥 denylist、validation 字段名解析顺序、CORS 默认策略、公开错误消息和 HTTP 响应行为不变。

#### Scenario: 配置校验集合不可被包内误写
- **WHEN** `common/runtime/config` 校验 log level、log format、Postgres driver、Postgres SSL mode、tracing exporter、production-like environment 或 insecure JWT secret
- **THEN** 校验逻辑 MUST 使用 `switch`、私有查询函数、局部构造或等价方式表达固定集合
- **AND** 系统 MUST NOT 暴露可被同包未来代码直接写入的 package-level map 作为这些固定集合的权威来源
- **AND** 合法值、非法值和错误消息 MUST 保持当前语义不变

#### Scenario: 默认 CORS 配置隔离共享 slice
- **WHEN** 调用方使用 `common/http/middleware.CORS()` 或 `CORSWithOptions` 创建 CORS middleware
- **THEN** middleware MUST 在构造时持有与 package-level 默认值和调用方传入 slice 隔离的配置副本
- **AND** 调用方后续修改其传入的 origins、methods、headers 或 exposed headers slice MUST NOT 改变已创建 middleware 的行为
- **AND** `CORS()` 的默认响应 MUST 继续使用 `Access-Control-Allow-Origin=*`、`Access-Control-Allow-Methods=GET,POST,PUT,PATCH,DELETE,OPTIONS` 和 `Access-Control-Allow-Headers=Authorization,Content-Type`

#### Scenario: validation request tag 顺序稳定且不可共享写入
- **WHEN** `common/validation` 从 struct field tag 推导请求字段名
- **THEN** tag 优先级和支持集合 MUST 保持当前顺序与语义
- **AND** 实现 MUST NOT 依赖可被同包未来代码修改的 package-level slice 作为共享底层状态

#### Scenario: 保留非只读集合变量需有理由
- **WHEN** 实现阶段发现 package-level var 不迁移
- **THEN** 该变量 MUST 不属于本次只读 map、slice 或默认 struct 风险范围，或具备明确保留理由
- **AND** 合理保留理由 MAY 包括 sentinel error、regexp 编译结果、Fx Module、`sync.Pool`、atomic counter 或需要运行时状态的对象
