## Why

当前本地 Compose 只把一组配置发布到 Nacos namespace `loca`。容器内运行需要使用 `postgres`、`redis`、`jaeger` 等 Compose DNS 名称，主机直接运行 user-service 则需要使用宿主机地址和映射端口；单一 Namespace 不能同时正确服务两种运行位置。

本地开发更需要每套配置可以独立查看、复制、校验和发布，而不是在排障时跨公共文件和 overlay 推导最终值。因此接受少量公共字段重复，为主机和 Docker 各保存一套完整三文档配置，并通过结构化测试约束本应一致的字段，避免无意漂移。

## What Changes

- **BREAKING** 本地 Nacos namespace 从单一 `loca` 拆分为 `loca-host` 和 `loca-docker`；主机与 Compose workload 必须选择对应 Namespace。
- 将本地 Nacos 配置迁移到 `deployments/nacos/local-host/` 与 `deployments/nacos/local-docker/`，每个目录都完整包含 `base.yaml`、`resources.yaml`、`user-service.yaml`。
- Compose 新增 `nacos-init-host` 与 `nacos-init-docker` 两个一次性服务，分别挂载一个完整目录，并使用原有 seed CLI 发布到 `loca-host` 与 `loca-docker`。
- `tools/nacos-config-seed` 保持单目录、单 Namespace 的 `--config-dir`、`--namespace`、`--group`、`--data-ids` 契约，不增加 `--config-root`、`--targets` 或多目标编排能力。
- 为两套完整配置增加防漂移测试：除明确允许的主机名、端口和环境观测地址外，共享配置必须保持一致。
- 保持真实 secret、Nacos 凭据和 production-like 凭据不进入 Git；本地已知开发占位值不得用于 production-like 环境。
- 更新 Compose、主机开发说明、配置诊断和迁移步骤，删除旧 `deployments/compose/nacos/init/` 配置来源。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `delivery-operations`: 新增双完整配置目录、独立本地 Namespace、双 Compose 初始化服务与防漂移契约。
- `shared-platform-primitives`: 明确主机和 Compose 继续使用固定三 dataId，只通过 Namespace 选择各自完整配置，runtime source 不感知仓库目录映射。

## Impact

- Go 代码：`tools/nacos-config-seed` 继续使用原有单目标 CLI；为配置一致性及 runtime 来源选择补充测试。
- 部署配置：新增 `deployments/nacos/local-host/`、`deployments/nacos/local-docker/`，以两个 Compose 一次性服务分别完成发布。
- 文档与规格：更新 `docs/DEVELOPMENT.md`、`deployments/compose/README.md`、能力地图和相关 OpenSpec 主规格。
- 安全：版本化配置不得包含真实 secret；发布日志不得输出文档内容或凭据。
- 兼容性：旧 `loca` Namespace 不再是本地开发契约；迁移需先发布并验证两个新 Namespace，再切换运行目标。
- 不影响 HTTP API、OpenAPI 生成物、Ent schema、Atlas migration、RBAC 数据模型、Kubernetes/Helm 生产 Namespace 或业务行为。
