## Context

AegisCore 当前由 `common/runtime/config` 读取 YAML 后通过 Viper 接受环境变量覆盖，user-service 的 JWT、具名 PostgreSQL/Redis 资源、Gin mode 等配置因此存在多个来源；timezone primitive 直接读取 `TZ`。Compose 又通过顶层 `environment`、服务级 `environment` 和 `${VAR}` 注入应用、数据库与 Grafana 凭据，Makefile、CI 和脚本也使用环境变量控制测试、镜像、migration 与 RBAC bootstrap。该模型与“配置文件是唯一配置来源”的目标冲突，且 secret、默认值和最终生效值难以统一审查。

本变更横跨 `common`、`user-service`、`deployments`、Makefile/CI 与文档。它不改变业务 API 或数据库结构，但会改变开发、测试、构建和发布入口，必须提供明确迁移路径并保持 secret 不入库。

## Goals / Non-Goals

**Goals:**

- 所有运行时与交付配置均来自显式配置文件，生产代码、Compose、启动脚本和 CI 不读取环境变量作为配置。
- 统一只接受一份完整 YAML 配置文件，保持严格字段校验和单次加载。
- Compose、Kubernetes 与 Helm 通过只读文件挂载传递完整配置，仓库仅提交无真实凭据的示例。
- 为环境变量读取、Compose `environment`/`env_file`/插值及脚本配置变量增加可自动执行的回归门禁。
- 保持现有 HTTP API、OpenAPI、Ent schema、Atlas migration 与业务行为不变。

**Non-Goals:**

- 不引入远程配置中心、动态配置刷新、Vault client、MQ 或新的外部依赖。
- 不改变 JWT、RBAC、资源池、观测或缓存的业务语义与字段层级。
- 不自动执行数据库 migration，也不把真实 secret 提交到 Git。
- 不将 user-service 私有配置下沉到 `common`，不新增 `internal/shared` 或 `integration` 业务边界。

## Decisions

### 1. 使用单一显式配置文件参数

user-service 的 `serve`、`rbac seed` 和 `rbac bootstrap-super-admin` 使用单一显式 `--config <path>`，该文件必须是当前环境完整 YAML 配置。CLI 在创建 Fx App 或 use case 前完成一次加载、默认值应用、严格字段校验和业务校验，composition root 只接收已解析对象。

选择单一完整文件而不是重复 `--config` 合并，是为了消除 ConfigMap/Secret overlay 边界和多来源拼装。未提供文件、文件不可读、字段未知或校验失败都在启动前失败。

### 2. loader 不绑定环境变量，timezone 纳入配置对象

`common/runtime/config` 保留 YAML 解码、默认值、严格字段和扩展配置能力，但删除 Viper 环境变量 prefix、key replacer、`AutomaticEnv`/`BindEnv` 等路径。进程 timezone 作为共享 runtime 配置字段加载并由 composition root 显式初始化，不再读取 `TZ`。

服务私有配置继续由 `user-service/internal/config` 拥有；`common` 不声明 JWT、RBAC、Ent 或具名资源业务用途。此选择保持现有架构边界，并让所有进程级行为都能从最终配置快照审查。

备选方案是保留 `TZ` 或少量 secret 环境变量作为例外；该方案仍会形成双来源并违反需求，因此不采用。

### 3. 敏感配置包含在受控完整 YAML 文件中

仓库提交本地示例与 `*.example.yaml`，实际 JWT secret、数据库密码和 Grafana 管理员凭据写入权限受控、被 `.gitignore` 排除或由外部 Secret 系统生成的完整配置文件。应用层沿用现有 production-like secret 强度校验；部署平台负责创建、挂载和轮换文件。

Kubernetes/Helm 使用外部 Secret 挂载唯一 `config.yaml`；Compose 使用 bind mount 或 Compose `secrets` 挂载唯一配置文件，不使用 `environment`、`env_file`、`${VAR}` 或 overlay 合并。

备选方案是 secret manager SDK 直接取值；这会引入新的外部依赖和动态失败模式，超出本次范围。后续若引入 secret manager，也应先渲染或挂载配置文件，不恢复进程环境变量入口。

### 4. 每个第三方容器使用其原生文件配置

PostgreSQL、Redis、Prometheus 与 Grafana 使用各自原生配置文件或受控初始化脚本读取挂载文件。需要初始化凭据而原生镜像只提供环境变量接口的组件，使用仓库拥有的最小 wrapper/初始化脚本从 secret 文件完成初始化，脚本不得读取环境变量配置，并保持镜像版本与安全基线可验证。

该方式避免为了“无环境变量”把第三方服务配置塞入 user-service YAML，也避免依赖宿主 shell 展开。wrapper 仅属于 `deployments`，不得进入 `common` 或业务 feature。

### 5. 工具与测试开关改为参数或配置文件

Makefile、CI、migration/OpenAPI/architecture lint、镜像验证和测试 harness 不再通过环境变量传递配置。可选行为使用显式命令参数、独立工具配置文件或专用 Make target。测试需要隔离配置时写入临时完整 YAML，而不是 `t.Setenv`。

### 6. 增加统一静态门禁

仓库级 lint 扫描生产 Go、脚本、Makefile、CI 和部署资产，拒绝配置用途的 `os.Getenv`/`LookupEnv`、Viper env binding、Shell 大写环境变量展开、Compose `environment`/`env_file` 和 `${VAR}`。允许构建系统固有且非配置的数据（例如 shell 局部变量、`PATH` 等）必须通过最小、可审查 allowlist 表达，避免简单正则误报所有 shell 变量。

## Risks / Trade-offs

- [Risk] secret 进入 YAML 文件会增加本地落盘暴露面 → 使用 `.gitignore`、示例占位符、最小文件权限、只读挂载和文档化轮换流程；验证仓库中不存在真实 secret。
- [Risk] 单一完整配置文件会复制非敏感字段 → 用示例、生成流程或环境配置仓库管理完整文件，并依靠严格字段校验暴露 drift。
- [Risk] 第三方镜像绕过默认 entrypoint 后初始化行为变化 → 优先使用原生配置文件；只有无文件接口的初始化步骤使用最小 wrapper，并增加全新 volume 与已有 volume 的 Compose 验证。
- [Risk] 删除 CI 环境开关可能增加默认测试耗时 → 将 Docker-backed 测试拆成显式 Make target/CI step，而不是运行时环境开关。
- [Risk] 现有部署命令立即失效 → 作为 breaking migration 一次性更新 Makefile、Compose、Kubernetes、Helm、CI 和文档，并提供旧入口失败提示而不是静默 fallback。
- [Trade-off] 文件挂载比环境变量注入更繁琐，但换取唯一来源、可审计性和跨环境一致性。

## Migration Plan

1. 扩展配置 schema，加入 timezone，完成单文件严格 loader 和 CLI 参数迁移。
2. 更新单元/集成测试，删除环境变量依赖，并增加缺失文件、未知字段和 secret 校验测试。
3. 更新 Docker/Compose 及第三方组件配置文件，验证全新 volume、重启和健康检查。
4. 更新 Kubernetes/Helm 为单个 Secret 文件投影与显式 `--config` 参数，执行 render、schema、server-side dry-run 和安全断言。
5. 更新 CI、Makefile、脚本和文档，启用静态门禁；运行相关包测试、architecture lint、Compose 检查和 `make verify`。
6. 发布时先生成实际完整配置文件并校验权限，再按 migration、RBAC seed/bootstrap、HTTP rollout 顺序切换；确认新版本稳定后删除旧环境变量和 overlay 配置。

回滚时回退应用与部署资产到上一版本，并恢复上一版本所需的环境变量注入。数据库 schema 和 HTTP API 未变化，因此不需要数据回滚；新配置文件可保留但不得被旧版本误读。

## Open Questions

无。实现阶段如发现某个第三方镜像无法在不设置环境变量的前提下安全初始化，必须在 `deployments` 中提供文件读取 wrapper 或更换为支持文件配置的等价启动方式，不得恢复环境变量例外。
