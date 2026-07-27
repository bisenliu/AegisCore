## 1. 两套完整配置目录

- [x] 1.1 创建 `deployments/nacos/local-host/` 与 `deployments/nacos/local-docker/`，每个目录完整保存 `base.yaml`、`resources.yaml`、`user-service.yaml`。
- [x] 1.2 将主机配置中的 PostgreSQL、Redis、OTLP 地址设置为宿主机地址和 Compose 映射端口，将 Docker 配置设置为 Compose DNS 和容器端口；其余共享字段保持一致。
- [x] 1.3 确认两套配置不依赖公共目录、overlay、target manifest、生成的 effective 文件或第四 dataId，并确认版本化文件不包含真实 production-like secret。

## 2. 单目标 Nacos 发布工具回归

- [x] 2.1 确认或恢复 `nacos-config-seed` 原有 `--config-dir`、`--namespace`、`--group`、`--data-ids` 单目录、单 Namespace CLI，删除本 change 曾引入的 `--config-root`、`--targets` 和多 Namespace 编排逻辑。
- [x] 2.2 保持单次发布在网络写入前读取并校验全部声明文档，按 dataId 顺序创建或复用一个 Namespace 并覆盖发布，日志不得输出凭据或文档内容。
- [x] 2.3 保持或补充 `tools/nacos-config-seed` 回归测试，覆盖单 Namespace 创建/复用、固定三 dataId 调用、重复运行、缺失或空文档、认证和发布失败诊断，并断言多目标 flags 不存在。
- [x] 2.4 运行 `go test ./...`（`tools/nacos-config-seed` module）并修复失败。

## 3. 配置一致性与严格校验

- [x] 3.1 新增结构化防漂移测试，解析两个目录的同名 YAML，仅排除精确允许的 Redis 地址、PostgreSQL host/port、OTLP endpoint 和经审查确认依赖运行位置的叶子字段后比较剩余配置。
- [x] 3.2 禁止防漂移测试使用顶层 section 或通配豁免，并断言两个目录的文件集合恰好是固定三 dataId。
- [x] 3.3 分别将两套三文档配置送入 user-service merge、strict decode、normalize 和 validate，确认两套配置都有效且未知键会失败。
- [x] 3.4 运行相关配置测试、`go test ./runtime/config/...`（`common` module）和 `go test ./internal/config ./cmd`（`user-service` module）并修复失败。

## 4. Compose 与主机运行来源

- [x] 4.1 修改 `deployments/compose/docker-compose.yml`，新增 `nacos-init-host` 与 `nacos-init-docker` 两个一次性服务，分别只读挂载 `local-host` 与 `local-docker` 目录，并通过原有单目标 CLI 发布到 `loca-host` 与 `loca-docker`。
- [x] 4.2 使 Compose user-service 与 RBAC seed 使用 `loca-docker` 并依赖 `nacos-init-docker` 成功；`nacos-init-host` 独立发布主机运行配置。
- [x] 4.3 更新 Compose/配置结构测试，断言两个初始化服务的目录挂载、Namespace、group、固定三 dataId、一次性退出策略和 Docker workload 依赖关系一致。
- [x] 4.4 更新主机运行命令和测试 fixture，使主机使用 `loca-host` 与宿主机 Nacos 地址，并验证配置中的 PostgreSQL、Redis、OTLP 地址使用 Compose 映射端口。
- [x] 4.5 在两个新 Namespace 可重复发布并通过诊断后，删除 `deployments/compose/nacos/init/` 旧平铺配置文件和所有 `loca` 本地运行引用，确认没有第二套 Git 配置来源。

## 5. 文档、规格与迁移说明

- [x] 5.1 更新 `docs/DEVELOPMENT.md` 和 `deployments/compose/README.md`，说明两个完整目录、两个 Compose 初始化服务、固定三 dataId、主机/Compose 命令、Git 权威来源和公共字段重复策略。
- [x] 5.2 记录从 `loca` 到 `loca-host`/`loca-docker` 的先发布、双端诊断、切换、回滚和旧 Namespace 人工清理步骤；明确工具不会自动删除旧 Namespace。
- [x] 5.3 更新 `docs/opsx/CAPABILITY_MAP.md`、`docs/ARCHITECTURE.md` 和相关部署说明，保持 `delivery-operations`、`shared-platform-primitives`、`tools`、`common` 与 user-service 配置归属一致。
- [x] 5.4 全仓搜索 `deployments/compose/nacos/init`、`AEGISCORE_NACOS_NAMESPACE=loca`、`--config-root`、`--targets`、manifest、overlay 和第四 dataId 引用，删除或迁移本 change 范围内的过时内容，并保留原有单目标 seed flags 文档。

## 6. 集成验证与收尾

- [x] 6.1 运行 `docker compose -f deployments/compose/docker-compose.yml config --quiet`，确认 Compose 可渲染且不泄漏真实 secret。
- [x] 6.2 启动本地 Nacos 或使用隔离测试环境分别执行 `nacos-init-host` 与 `nacos-init-docker`，确认两个 Namespace 中固定三 dataId 均按预期发布，并验证每个服务可幂等重跑。
- [x] 6.3 分别以 `loca-host` 和 `loca-docker` 运行 user-service `config sources`、`config validate`、`config render`；确认 Namespace、固定三 dataId、环境地址和脱敏结果正确。
- [x] 6.4 运行 `make user-service-architecture-lint` 并修复失败。
- [x] 6.5 运行 `openspec validate add-nacos-environment-targets`、`openspec list --specs` 和 `openspec validate --specs` 并修复失败。
- [x] 6.6 检查 `git diff`，确认只包含本 change 预期代码、配置、部署资产、文档和 OpenSpec artifacts。
- [x] 6.7 将本次预期变更加到暂存区后运行 `make lint`，未通过时修复并重新暂存后重跑。
- [x] 6.8 保持本次预期变更处于暂存区后运行 `make verify`，未通过时修复并重新暂存后重跑。
- [x] 6.9 所有实现、文档和验证完成后，将对应 checkbox 立即更新为 `- [x]`；任一要求的验证未通过或未运行时不得将 change 视为完成。
