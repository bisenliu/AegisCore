## ADDED Requirements

### Requirement: Document exported Go API with godoc comments
系统 SHALL 为仓库内手写 Go 源码中的所有导出类型、函数、方法、接口、常量和变量提供符合 Go godoc 规范的注释。注释 MUST 以被注释的标识符名称开头，并准确说明该标识符的用途、主要行为、关键参数、返回结果以及调用方需要感知的重要错误或边界条件。

#### Scenario: Exported symbol has godoc comment
- **WHEN** 开发者或自动化工具审查 `common/` 或 `user-services/` 中的手写 Go 源码
- **THEN** 每个导出的类型、函数、方法、接口、常量和变量 MUST 具有 godoc 注释
- **THEN** godoc 注释 MUST 以对应导出标识符名称开头

#### Scenario: Public behavior is described accurately
- **WHEN** 导出函数、方法或接口存在关键参数、返回值或重要错误语义
- **THEN** godoc 注释 MUST 描述调用方需要理解的行为和错误边界
- **THEN** godoc 注释 MUST NOT 承诺当前代码或主规格尚未实现的行为

### Requirement: Explain complex logic and non-obvious decisions
系统 SHALL 对复杂业务逻辑、关键分支判断、边界条件处理和不直观实现细节补充必要注释。注释 MUST 解释为什么采用该处理方式，并避免重复代码表面的控制流或赋值含义。

#### Scenario: Complex branch is documented
- **WHEN** 业务逻辑包含错误映射、认证边界、配置 fallback、资源生命周期、事务边界、请求校验或响应转换等关键分支
- **THEN** 实现 MUST 在相关位置提供简洁注释说明该分支的业务原因或运行时约束
- **THEN** 注释 MUST NOT 仅复述代码已经清楚表达的条件判断

#### Scenario: Boundary condition is documented
- **WHEN** 实现处理空值、缺失配置、默认实例名、超时、重复资源、not found、唯一性冲突或外部输入边界
- **THEN** 注释 MUST 说明该边界条件的来源、兼容性考虑或对调用方行为的影响

### Requirement: Document magic numbers defaults and strategy parameters
系统 SHALL 对手写 Go 源码中的魔法数字、特殊阈值、默认值、超时预算、TTL、容量限制和策略参数提供注释、命名或邻近文档说明。说明 MUST 明确该值的业务含义、来源、选择依据或与运行时契约的关系。

#### Scenario: Runtime default has semantic explanation
- **WHEN** 代码定义或使用运行时默认值、fallback、超时预算、TTL 或限制阈值
- **THEN** 实现 MUST 通过注释、常量名称或邻近文档说明该值代表的语义
- **THEN** 实现 MUST 遵循 `runtime-constant-governance` 的所有权边界，不得为单一局部场景新增无意义的全局常量

#### Scenario: Example value is intentionally local
- **WHEN** 值只用于测试、Swagger example、脚本 usage、局部格式字符串或一次性示例
- **THEN** 实现 MAY 保持局部字面量
- **THEN** 如果该值容易被误认为生产默认值，注释 MUST 明确其示例或测试语义

### Requirement: Verify comment governance with lint
系统 SHALL 在注释治理实现完成后运行 `golangci-lint`，并修复与注释覆盖、godoc 风格或注释一致性相关的问题。验证结果 MUST 能证明注释治理没有引入格式或静态检查回归。

#### Scenario: Lint reports comment issues
- **WHEN** `golangci-lint` 输出注释缺失、注释格式、导出标识符文档或 godoc 风格问题
- **THEN** 实现 MUST 修复这些注释相关问题
- **THEN** 修复 MUST 保持外部 HTTP API、配置、数据库 schema、生成代码和运行时行为不变

#### Scenario: Lint reports unrelated historical issues
- **WHEN** `golangci-lint` 输出与注释治理无关的历史问题
- **THEN** 实现 MUST 在结果中区分这些问题与本变更范围
- **THEN** 实现 SHOULD 只修复本变更引入或注释相关的问题，避免扩大非功能性变更范围
