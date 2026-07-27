## Context

当前 `deployments/compose/nacos/init/` 保存 `base.yaml`、`resources.yaml` 和 `user-service.yaml`，`tools/nacos-config-seed` 通过一个目录、一个 Namespace 和一组 dataId 发布它们。Compose 内资源地址使用容器 DNS，而主机直接执行 `make user-service-run` 需要 `127.0.0.1` 和 Compose 映射端口，导致两种运行位置不能共享同一份最终配置。

现有 runtime 已支持 `AEGISCORE_NACOS_NAMESPACE` 选择 Namespace，并默认按 `base.yaml`、`resources.yaml`、`user-service.yaml` 读取和合成文档。本次不改变 runtime merge 管线，而是为两种本地运行位置提供可独立查看和发布的完整配置集合。

本次 change 影响 `deployments/nacos/`、`deployments/compose/docker-compose.yml`、本地开发文档、配置测试与 `openspec`。`tools/nacos-config-seed` 保持现有单目录、单 Namespace CLI；`common/runtime/config/nacos` 继续拥有 Nacos runtime source，`user-service/internal/config` 继续拥有服务配置 defaults、normalize 和 validate。

## Goals / Non-Goals

**Goals:**

- 为主机和 Docker 各提供一套完整、可独立查看、校验和发布的三文档配置。
- 将 `local-host` 和 `local-docker` 配置分别发布到 `loca-host`、`loca-docker`。
- 通过两个 Compose 一次性服务分别调用现有 seed CLI 发布主机和 Docker 配置。
- 通过结构化测试约束两套配置的允许差异，避免重复公共字段无意漂移。
- 保持 runtime document source、deep merge、strict decode、raw digest、normalize 和 validate 管线不变。
- 防止版本化配置和发布日志泄漏生产 secret 或 Nacos 凭据。

**Non-Goals:**

- 不采用 `common/`、`overlays/`、`targets/` 或 target manifest 分层。
- 不新增 `environment.yaml` 或改变既有三 dataId 默认顺序。
- 不为 seed 工具增加 `--config-root`、`--targets`、目录扫描或多目标发布能力。
- 不新增环境变量对任意业务配置字段的覆盖层。
- 不恢复本地 YAML runtime source或文件 fallback；运行时仍以 Nacos 为配置来源。
- 不让 seed 工具合成 YAML、理解 user-service Go 配置类型或执行业务 defaults/normalize/validate。
- 不实现 Nacos 动态监听、配置热更新、版本回滚或跨 Namespace 事务。
- 不修改 HTTP API、OpenAPI、数据库 schema、Atlas migration、RBAC 或观测资产。

## Decisions

### Decision: 保存两套完整三文档配置

配置目录固定为：

```text
deployments/nacos/
├── local-host/
│   ├── base.yaml
│   ├── resources.yaml
│   └── user-service.yaml
└── local-docker/
    ├── base.yaml
    ├── resources.yaml
    └── user-service.yaml
```

每个目录都是可独立发布的完整配置集合，不依赖目录外公共文件、overlay、manifest 或生成后的 effective 文件。主机目录直接包含宿主机地址和 Compose 映射端口；Docker 目录直接包含 Compose DNS 与容器端口。

该选择有意接受 `app`、runtime、server、日志、资源池、auth、RBAC 和 Ent 等公共字段重复，换取配置审查、控制台比对、故障诊断和单环境复制的直接性。

备选方案：公共配置加环境 overlay。该方案减少文本重复，但单个环境的最终配置需要跨四份文档推导，用户已明确不采用。

### Decision: 两个 Compose 服务分别调用原有单目标 seed CLI

Compose 定义 `nacos-init-host` 和 `nacos-init-docker` 两个一次性服务。二者复用同一个 `nacos-config-seed` 镜像，分别只读挂载 `deployments/nacos/local-host/` 与 `deployments/nacos/local-docker/` 到容器内配置目录，并分别设置 `AEGISCORE_NACOS_NAMESPACE=loca-host`、`AEGISCORE_NACOS_NAMESPACE=loca-docker`。group 都为 `AEGISCORE`，dataId 都为 `base.yaml,resources.yaml,user-service.yaml`。

seed 工具继续使用原有 `--config-dir`、`--namespace`、`--group`、`--data-ids` 及对应环境变量表达单次发布。每个进程只校验、创建和发布一个 Namespace；多环境编排由 Compose 服务图负责，不进入 Go CLI。

备选方案：给 seed 工具增加 `--config-root` 与 `--targets`。该方案把本地多环境部署编排引入通用发布工具，增加不必要的 CLI 和测试面，最终实现不采用。

### Decision: Runtime 继续使用固定三 dataId

两个 Namespace 都发布并加载：

```text
base.yaml -> resources.yaml -> user-service.yaml
```

Compose user-service 与 RBAC seed 设置 `AEGISCORE_NACOS_NAMESPACE=loca-docker`；主机命令设置 `AEGISCORE_NACOS_NAMESPACE=loca-host`。二者都使用既有三 dataId 默认顺序，不设置第四文档，也不要求 runtime 读取目录映射。

`common/runtime/config/nacos` 不导入发布目标类型，不读取 Git 目录，不推断主机或容器环境，不创建 Namespace，也不改写资源地址。`user-service/internal/config` 不增加 host/docker 判断。

### Decision: 用允许差异清单治理重复字段

测试分别结构化解析两个目录中的同名 YAML 文档，并在删除明确允许差异的字段后比较剩余配置。允许差异必须使用精确字段路径声明，初始只包括 Redis 地址、PostgreSQL host/port、OTLP endpoint，以及经审查确认确实依赖运行位置的少量字段。

新增差异路径必须同时修改测试允许清单并接受 review；不得用整段 `resources`、`observability` 或任意通配路径跳过比较。测试还必须确认两边文件集合恰好是固定三 dataId，且每套文档都能通过 user-service 严格加载和校验。

备选方案：逐字节比较公共文档。由于环境地址嵌在完整文档中，该方案无法表达合法差异。手工 review 则无法形成稳定门禁。

### Decision: 每个初始化服务独立失败并幂等重试

每个 seed 进程沿用现有行为：先读取其 `--config-dir` 下声明的 dataId，再创建或复用单个 `--namespace` 并按 dataId 顺序覆盖发布。相同服务以相同输入重跑必须幂等收敛。

两个初始化服务之间不提供跨 Namespace 事务。任一服务失败时必须保留自己的精确 Namespace/group/dataId 诊断，且不得删除另一服务已经发布的配置。Compose 内 Docker workload 只在 `nacos-init-docker` 成功后启动；`nacos-init-host` 的结果独立用于主机运行。

### Decision: 敏感值不进入 Git 或发布日志

Nacos Admin API 的 username/password 只通过进程环境或 Secret 注入；工具不得输出凭据、配置内容或 effective settings，只报告目录、Namespace、group、dataId 和结果。

两个版本化目录不得包含真实 production-like secret。为本地校验使用的固定开发占位值必须明确标注为公开、仅限本地且不可复用到 production-like 环境。production-like secret 的受控写入、Secret manager 集成和轮换不属于本次 change。

## Risks / Trade-offs

- [风险] 两套完整配置的公共字段可能漂移。 -> 缓解：结构化比较同名文档，仅允许精确字段路径差异；两套配置都执行严格加载和校验。
- [风险] 允许差异清单过宽会掩盖漂移。 -> 缓解：禁止顶层 section 和通配豁免，每个新增路径必须单独 review。
- [风险] 两个初始化服务可能一个成功、一个失败。 -> 缓解：服务状态和日志可独立诊断，每个单目标发布均可幂等重跑；Docker workload 依赖其实际消费的 `nacos-init-docker`。
- [风险] 旧 `loca` 使用者仍按旧环境变量启动。 -> 缓解：先发布两个新 Namespace，再更新 Compose 和主机文档；通过 `config sources` 检查实际 Namespace。
- [风险] 本地开发占位 secret 被复制到生产环境。 -> 缓解：两个目录明确限定为 local，加入内容断言，生产资产不得引用它们。
- [权衡] 公共字段会重复。 -> 缓解：这是为了让每套配置完整可见而接受的成本，由自动测试代替人工同步。

## Migration Plan

1. 新增两个完整配置目录，保留旧 `loca` Nacos 内容不动。
2. 确认 seed 工具恢复并保持原有单目录、单 Namespace CLI，相关回归测试通过。
3. 使用两个独立 seed 调用向现有本地 Nacos 发布 `loca-host` 与 `loca-docker`，分别运行 `config sources`、`config validate` 和脱敏后的 `config render`。
4. 修改 Compose，使用 `nacos-init-host` 与 `nacos-init-docker` 分别挂载和发布两个目录，容器 workload 切换到 `loca-docker`。
5. 更新主机运行命令，使其连接宿主机 Nacos 地址并选择 `loca-host`；验证可连接 Compose 暴露的 PostgreSQL、Redis 和 OTLP 端口。
6. 更新文档和测试后删除旧 `deployments/compose/nacos/init/`；旧 `loca` Namespace 不由工具自动删除，可在确认无使用者后人工清理。
7. 运行工具测试、配置防漂移测试、user-service 配置测试、Compose 渲染、架构检查、lint 和完整 verify。

回滚方式：在旧 `loca` 内容仍保留时，将 workload Namespace 恢复为旧值并回滚 Compose 与配置目录变更。新 Namespace 无需删除即可完成运行时回滚，确认不再使用后再由运维人工清理。

## Validation

- `go test ./...`（`tools/nacos-config-seed` module）。
- 运行两套配置结构化防漂移测试和 user-service strict load/validate 测试。
- `go test ./runtime/config/...`（`common` module）。
- `go test ./internal/config ./cmd`（`user-service` module）。
- `docker compose -f deployments/compose/docker-compose.yml config --quiet`。
- 启动本地 Nacos 后执行两个 Compose 初始化服务，并分别执行 `config sources`、`config validate` 和 `config render`。
- `make user-service-architecture-lint`、`make lint` 和 `make verify`。

## Open Questions

无。
