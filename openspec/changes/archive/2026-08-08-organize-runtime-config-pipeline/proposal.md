## Why

`common/runtime/config` 已经承担配置来源、YAML 合并、严格解码、默认值、校验、编码、脱敏和渲染等共享原语，但包级导航和测试布局仍偏向单一大文件。审查者需要同时阅读 source、merge、decode、validation 和 render 多个入口，才能确认服务扩展配置应如何按固定顺序组合，也难以快速定位 runtime、server、log 与 observability 校验职责。

## What Changes

- 更新 `shared-platform-primitives` OpenSpec delta，固化配置 source、merge、raw digest、strict decode、defaults、normalize、validate、encode、redact 和 render 的固定 pipeline，以及服务扩展配置的显式使用方式。
- 保持 `common/runtime/config` 的 package、`Config` 类型和导出函数不变，将 validation error/aggregate、runtime、server、log 和 observability 校验整理到职责清晰的同 package 文件。
- 将覆盖多个关注点的 config 测试拆分为 defaults、strict decode、server validation 和 observability validation 文件，并保留默认值、严格未知键、错误路径和聚合错误语义。
- 新增 `doc.go` 和 executable example，用内存 `DocumentSource` 展示 `LoadSource`、`DeepMergeYAML`、`DecodeStrict`、`Validate`、`EncodeSettings` 和 `RenderYAML` 的调用顺序。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 补充 `common/runtime/config` 的完整配置加载、解码、校验、编码、脱敏和渲染 pipeline，以及服务扩展配置必须显式组合 defaults、normalize 和 validate 的边界。

## Impact

- 影响代码：`common/runtime/config` 包内文件组织、包文档、示例测试和相关普通测试。
- 影响规格：新增 `openspec/changes/organize-runtime-config-pipeline/specs/shared-platform-primitives/spec.md` delta。
- API 兼容性：保持原 package、`Config` 类型、导出函数、默认值、字段路径、错误聚合、strict decode、digest 和 render 语义。
- 边界：不把 user-service auth、RBAC、Ent、rate limit、具名资源必需名称或服务私有敏感路径移入 common。
- 验证：需要通过 config 包普通测试、race 测试、go vet、executable examples、`make common-test`、`make user-service-architecture-lint`、`make lint` 和 `make verify`。
