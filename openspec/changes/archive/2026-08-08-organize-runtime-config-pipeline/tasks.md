## 1. 规格与现状确认

- [x] 1.1 确认需求归属 `shared-platform-primitives`，且本次只整理 `common/runtime/config` 的 pipeline 文档、同 package 文件布局和测试布局。
- [x] 1.2 确认不改变 `Config` 类型、导出函数、默认值、字段路径、错误聚合、strict decode、digest 或 render 语义。

## 2. OpenSpec delta

- [x] 2.1 新增 `openspec/changes/organize-runtime-config-pipeline/specs/shared-platform-primitives/spec.md`，固化 source、merge、raw digest、strict decode、defaults、normalize、validate、encode、redact 和 render 调用顺序。
- [x] 2.2 在 delta 中声明服务扩展配置必须显式提供 defaults、normalize 和 validate，`common/runtime/config` 不自动发现服务 hook、不承载服务私有业务字段或敏感路径策略。

## 3. config 包文件整理

- [x] 3.1 新增 `doc.go`，说明配置文档来源到有效配置输出的固定 pipeline、服务扩展方式和 common 业务中立边界。
- [x] 3.2 保持 `validation.go` 承载 validation error/aggregate、入口和共享 helper，将 runtime、server、log、observability 校验拆到职责文件。
- [x] 3.3 新增 `example_test.go`，用内存 `DocumentSource` 展示 `LoadSource`、`DeepMergeYAML`、`DecodeStrict`、`Validate`、`EncodeSettings` 和 `RenderYAML`。
- [x] 3.4 将原大测试拆分为 defaults、strict decode、server validation 和 observability validation 文件，保留同包测试 helper。

## 4. 验证

- [x] 4.1 运行 `go test ./common/runtime/config`。
- [x] 4.2 运行 `go test -race ./common/runtime/config`。
- [x] 4.3 运行 `go vet ./common/runtime/config`。
- [x] 4.4 运行 `make common-test`。
- [x] 4.5 运行 `make user-service-architecture-lint`。
- [x] 4.6 运行 `make lint`。
- [x] 4.7 运行 `make verify`。

## 5. 收尾检查

- [x] 5.1 检查本次 diff 只包含预期 OpenSpec、config 包文档、示例、文件整理和测试拆分。
- [x] 5.2 将已完成任务逐项改为 `- [x]`，并确认不做 archive。
