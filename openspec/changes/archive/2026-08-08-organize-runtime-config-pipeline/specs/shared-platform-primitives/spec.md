## ADDED Requirements

### Requirement: Runtime config pipeline 文档与布局

`common/runtime/config` MUST 作为跨服务共享、业务中立的配置原语维护清晰的 source、merge、decode、defaults、validation、encode、redact 和 render 导航。实现 MAY 在同 package 内按职责拆分文件和测试，但 MUST 保持原 package、`Config` 类型、导出函数、默认值、字段路径、错误聚合、strict decode、raw digest、effective encode、redact 和 render 语义兼容。

#### Scenario: 固定配置加载管线可从包文档导航

- **WHEN** 调用方阅读 `common/runtime/config` 包文档或 executable example
- **THEN** 文档 MUST 展示通过 `DocumentSource` 调用 `LoadSource` 取得 raw merged settings 和 source metadata
- **AND** 文档 MUST 展示 `LoadSource` 内部按文档顺序执行 YAML deep merge，并在 defaults 和 normalize 前基于 raw settings 计算 `SourceMetadata.Digest`
- **AND** 文档 MUST 展示调用方随后显式执行 `DecodeStrict`、`Validate`、`EncodeSettings`、可选 `RedactSettings` 和 `RenderYAML`

#### Scenario: 服务扩展配置显式组合

- **WHEN** 服务在共享 `Config` 之外增加服务私有配置字段
- **THEN** 服务 MUST 通过 `DecodeOptions` 显式提供完整 defaults、可选 normalize 和最终 validate
- **AND** `DecodeStrict` MUST 在 normalize 和 validate 前拒绝未知配置键并报告完整叶子路径
- **AND** shared loader MUST NOT 通过目标类型自动发现 `ConfigDefaults()`、`ApplyDefaults()` 或其他服务 hook
- **AND** user-service 的 auth、RBAC、Ent、rate limit、具名资源必需名称和业务校验 MUST 留在 `user-service/internal/config` 或所属 feature 边界

#### Scenario: validation 文件职责清晰且 API 兼容

- **WHEN** config package 维护 validation error/aggregate、runtime、server、log 和 observability 校验
- **THEN** 这些职责 MUST 通过清晰的同 package 文件组织表达
- **AND** 文件拆分 MUST NOT 创建 config 子包、改变调用方 import path、移除导出 helper 或改变错误文本和字段路径

#### Scenario: 测试与示例覆盖 pipeline 而不污染生产 API

- **WHEN** 为 config package 增加或重排测试和 executable examples
- **THEN** 测试 MUST 覆盖 defaults、strict decode、server validation 和 observability validation 的既有行为
- **AND** executable example MUST 使用本地内存 `DocumentSource` 或局部 YAML fixture，不访问公网、Nacos、PostgreSQL、Redis、user-service feature 或部署资产
- **AND** 正式代码 MUST NOT 仅为测试便利新增全局可变函数、测试 flag、`NewXForTest` 或无运行时职责的 adapter
