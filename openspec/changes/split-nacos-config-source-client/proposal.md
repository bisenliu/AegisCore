## Why

`common/runtime/config/nacos/client.go` 同时承载 server failover、认证 token、HTTP 请求/响应处理、endpoint 解析和 Nacos 文档读取流程。行为已经有测试保护，但聚合在单文件后不利于定位失败来源，也不利于未来只审查某一类边界，例如认证或响应体上限。

当前 Nacos source 与 `common/runtime/config.LoadSource` 的组合只在普通测试中体现，缺少可读的本地 `httptest.Server` 示例来固化多 dataId 顺序加载、合并和 metadata 行为。需要在不改变导出 API 和环境变量契约的前提下，把 package 内职责拆清楚，并补充示例作为使用边界。

## What Changes

- 将 Nacos v3 loader 的实现拆分为同 package 聚焦文件：failover、auth、HTTP transport、endpoint 解析与核心 client 类型。
- 保留 `Env`、`NewSource`、`Source.LoadDocuments`、`LoadEnv`、`AEGISCORE_NACOS_*` 环境变量、默认 dataId 顺序、Bearer token、response size limit、safe error message 和总 timeout 分配行为。
- 新增 `common/runtime/config/nacos/doc.go` 描述该 package 是 `common/runtime/config` 的 Nacos document source adapter，而不是配置核心或动态刷新能力。
- 新增基于 `httptest.Server` 的 example，展示 `nacos.NewSource` 与 `config.LoadSource` 组合加载多 dataId 并生成 metadata，不连接真实 Nacos。
- 继续通过现有测试覆盖多服务器失败聚合、timeout 预算、认证复用、endpoint 解析、响应体上限和敏感错误裁剪。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 明确 Nacos source adapter 的内部职责边界、失败语义和与 `DocumentSource`/`LoadSource` 的组合方式。
- `delivery-operations`: 明确 Nacos adapter 示例和普通测试属于 common module 质量门禁，不需要真实 Nacos、配置 watch 或外部 SDK。

## Impact

- Go 代码：仅调整 `common/runtime/config/nacos` package 内文件组织和示例测试，导出 API 不变。
- 测试：新增 example，保留并运行 Nacos package 普通测试、race 测试和 vet；按范围运行 common 相关门禁。
- 文档与规格：新增 OpenSpec change delta；不归档主规格。
- 部署与数据：不修改 Compose、Helm、Kubernetes、数据库 schema、migration、OpenAPI 或 RBAC 数据。
- 兼容性：不改变 `AEGISCORE_NACOS_*` 环境变量、namespace/group/dataId 契约、Nacos v3 HTTP endpoint 或错误文本的公开语义。
