## ADDED Requirements

### Requirement: 显式配置来源与加载管线

系统 MUST 将配置文档来源、文档合成、严格解码和服务配置策略表达为显式边界。`common/runtime/config` MUST 只拥有业务中立的配置 schema、document contract、deep merge、raw digest、strict decode、encode、render、redact 和通用校验原语；具体 Nacos 环境、认证、client、failover 和文档读取 MUST 位于 Nacos source adapter。服务 MUST 显式组合 defaults、normalize 和 validate，shared loader MUST NOT 通过服务类型的隐式接口自动发现这些行为。

#### Scenario: Nacos 缺省文档来源

- **WHEN** 服务已设置必需 Nacos 环境变量但未设置 `AEGISCORE_NACOS_DATA_IDS`
- **THEN** Nacos source MUST 按 `base.yaml`、`resources.yaml`、`<service>.yaml` 的稳定顺序读取文档
- **AND** 显式 `AEGISCORE_NACOS_DATA_IDS` MUST 继续按声明顺序读取，认证、timeout 和 server failover 行为 MUST 保持不变

#### Scenario: 文档合成与严格解码

- **WHEN** source 返回多份 YAML 文档
- **THEN** pipeline MUST 按文档顺序递归合并 map，后者 MUST 覆盖相同 scalar、slice 或显式 null，未被覆盖的嵌套字段 MUST 保留
- **AND** pipeline MUST 依次执行 raw settings 合成、raw digest、strict decode、normalize 和 validate
- **AND** 未声明配置键 MUST 在返回 typed config 前失败并报告完整叶子路径

#### Scenario: 显式服务配置策略

- **WHEN** user-service 解码服务私有配置
- **THEN** 调用点 MUST 显式提供完整默认配置、可选 normalize 和最终 validate
- **AND** shared loader MUST NOT 依赖 `ConfigDefaults()`、`ApplyDefaults()` 或其他按目标类型自动发现的服务 hook
- **AND** user-service 的 auth、RBAC、Ent、具名资源默认值和业务校验 MUST 留在 `user-service/internal/config`

#### Scenario: Raw digest 与 effective render

- **WHEN** pipeline 成功合成 raw settings
- **THEN** `SourceMetadata.Digest` MUST 基于 defaults 和 normalize 前的 raw merged settings 生成稳定摘要
- **AND** 默认值、normalize 或 typed config 的后续修改 MUST NOT 反向改变已记录的 source digest
- **WHEN** CLI 渲染 effective settings
- **THEN** 系统 MUST 从最终 typed config 编码可读 duration 和 mapstructure 字段，并在输出前脱敏 JWT、Redis、PostgreSQL 及调用方声明的敏感路径
- **AND** effective render MUST NOT 泄漏原始 secret

#### Scenario: 新增配置来源

- **WHEN** 测试或未来服务使用非 Nacos 配置来源
- **THEN** 来源 MUST 能通过业务中立的 document source contract 接入同一 merge、digest、decode、normalize 和 validate 管线
- **AND** 新来源 MUST NOT 要求修改 Nacos adapter 或把服务业务配置加入 `common`
