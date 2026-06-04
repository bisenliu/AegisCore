## ADDED Requirements

### Requirement: Organize constants by ownership boundary
系统 SHALL 按 capability 和所有权边界组织常量，不得因为值是常量就集中到单一全局 constants 包。跨模块或外部可观察契约常量 MUST 位于拥有该契约的公共包或能力包中；能力内部业务规则和实现 fallback MUST 就近定义在对应 domain、service、repository、bootstrap、config、middleware 或 response 包内。

#### Scenario: Cross-module contract constants are owned by common packages
- **WHEN** 常量表达认证传输、API 响应码、trace header、配置加载策略、运行时资源名或共享基础设施契约
- **THEN** 常量 MUST 定义在拥有该契约的 `common` 子包中
- **THEN** 服务侧代码 MUST 复用该公共常量，除非 Go struct tag 或外部文档格式无法引用常量

#### Scenario: Business constants stay near the business boundary
- **WHEN** 常量表达用户状态、用户字段长度、认证会话 TTL fallback、Redis session key 格式或业务错误
- **THEN** 常量 MUST 定义在对应业务能力的 domain、DTO、schema、service 或 repository 边界附近
- **THEN** 实现 MUST NOT 为这些服务内业务规则新增跨项目全局 constants 包依赖

#### Scenario: Local scenario values are not centralized
- **WHEN** 值只用于单个测试、Swagger example、脚本 usage、局部格式字符串或一次性示例
- **THEN** 实现 MAY 保持就近字面量
- **THEN** 实现 MUST NOT 为减少字面量数量而牺牲可读性或引入无意义公共 API

### Requirement: Prevent duplicated defaults from drifting silently
系统 SHALL 对同一语义的默认值或业务阈值建立单一来源或显式一致性保护。实现 MUST 区分部署示例值、缺失配置 fallback、测试场景值和外部协议示例，不得让语义相同的值在多个文件中无说明地漂移。

#### Scenario: Same semantic default has one owner
- **WHEN** 多个包需要同一个默认值或阈值
- **THEN** 实现 MUST 明确该值的 owner 包
- **THEN** 其他包 MUST 复用 owner 常量或通过测试验证与 owner 保持一致

#### Scenario: Example value differs from fallback intentionally
- **WHEN** YAML 示例、Swagger example 或文档示例与代码 fallback 不一致
- **THEN** 实现 MUST 能从命名、注释、测试或文档中判断该差异是部署示例还是安全 fallback
- **THEN** 实现 MUST NOT 让维护者误以为二者是同一个默认值

#### Scenario: Generated or tag-only values are protected without overengineering
- **WHEN** Go struct tag、Swagger annotation 或生成工具输入需要字符串字面量
- **THEN** 实现 MAY 保留必要字面量
- **THEN** 实现 SHOULD 用邻近常量、测试或规格约束防止其与公共契约漂移

### Requirement: Document constant refactoring review output
系统 SHALL 在常量重构审查中输出问题清单、影响范围、风险说明、统一管理判断依据、推荐组织方式和具体重构建议。审查结论 MUST 明确哪些常量适合集中，哪些应保持就近定义。

#### Scenario: Review shutdown timeout constants
- **WHEN** 审查 `user-services/cmd/main.go` 与 `user-services/internal/bootstrap/server.go` 的关闭超时
- **THEN** 输出 MUST 区分 Fx app stop budget 与 HTTP server graceful shutdown budget
- **THEN** 输出 MUST 说明命名和取值不一致是否会导致理解、维护或行为风险
- **THEN** 输出 MUST 给出统一命名和预算关系建议

#### Scenario: Review project-wide constants
- **WHEN** 审查项目中的超时、端口、路径、业务阈值、默认值、错误码、配置 key 和资源名
- **THEN** 输出 MUST 按类别说明当前组织问题
- **THEN** 输出 MUST 说明影响范围与风险
- **THEN** 输出 MUST 给出按当前项目结构落地的文件或模块建议
