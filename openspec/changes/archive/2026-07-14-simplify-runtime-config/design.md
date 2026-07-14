## Context

当前 runtime 配置把跨服务核心字段、Redis/PostgreSQL 资源、feature cache、文件日志、pprof 和入口代理策略混在同一根对象中。多个公共 provider 和 user-service 调用方直接依赖这些旧字段，因此单独删除核心类型会立即破坏 `common/runtime/timezone`、datastore、testing、logger 和 user-service 编译。

迁移禁止保留兼容字段或类型别名。为满足 OpenSpec 的全量验证和归档门槛，所有类型定义、调用方、配置样例、部署资产和文档必须在同一个 change 中按依赖顺序完成，再统一运行仓库门禁。

## Goals / Non-Goals

**Goals:**

- 让核心 `Config` 只拥有 `App`、`Server`、`Log` 和 `Observability`。
- 让 Redis/PostgreSQL 类型可跨服务复用，但资源声明和必需资源校验归属具体服务。
- 让旧配置字段严格失败，并保留服务自有扩展配置能力。
- 完成 datastore、user-service、HTTP、cache、logging、tracing、pprof 和部署配置的原子迁移。
- 保证最终状态通过相关单元测试、OpenSpec 校验、架构 lint、`make lint` 和 `make verify`。

**Non-Goals:**

- 不保留旧字段、旧类型、旧 YAML、字段别名或自动迁移。
- 不实现入站 gRPC server、远程配置中心、动态日志级别或完整诊断平台。
- 不新增其他资源类型，不修改业务 API、Ent schema、Atlas migration 或 OpenAPI 契约。

## Decisions

### Decision: 使用一个原子 OpenSpec change

`simplify-runtime-config` 包含全部实现、配置、部署、文档和规格迁移。内部子任务由独立 agent 顺序完成并逐项暂存，但只在全部子任务完成后执行一次仓库级 lint、verify 和归档。

备选方案是继续拆成多个独立 breaking change，但首个删除旧类型的 change 无法在调用方迁移前编译；增加兼容层又违反明确的无兼容目标，因此拒绝。

### Decision: 核心配置与资源配置分离

`common/runtime/config.Config` 只声明四个稳定分组。`common/runtime/resources` 提供 Redis/PostgreSQL 类型、默认值和通用校验。user-service 通过自己的根 Config 组合核心配置、ResourcesConfig 和业务配置，并负责必需具名资源和 feature cache 的语义校验。

### Decision: 新类型先建立，旧契约最后彻底清除

实现顺序先建立 resources 和服务自有配置，再迁移 datastore、providers、HTTP、cache 和 observability 调用方，最后完成严格 unknown key 检查和仓库级旧字段清理。最终提交中不存在过渡兼容层；中间工作状态不作为可发布版本。

### Decision: pprof 使用独立诊断监听

pprof 不进入 YAML，也不注册到业务 Gin router。服务可通过显式诊断启动逻辑或 `PPROF_ENABLED` 启用独立 server，`PPROF_ADDR` 默认 `127.0.0.1:6060`，production 环境拒绝非 loopback 地址。

### Decision: 日志和 tracing 保持云原生最小契约

logger 只输出 stdout/stderr，通过 `logger`、`component` 和关联字段分类，不再进行应用内文件拆分或轮转。Tracing 只支持 OTLP endpoint、insecure 和 sample ratio，不保留 exporter 选择层。

### Decision: 进程时区归属平台环境

删除 `system.timezone` 后，`common/runtime/timezone` 不再读取核心 Config。进程优先使用平台注入的 `TZ` 环境变量，未设置时继续使用 runtime primitive 的稳定默认值；服务启动日志记录实际 `time.Local`。该边界避免把主机级进程设置重新包装成核心 YAML 字段。

## Risks / Trade-offs

- [Risk] 原子 change 影响范围较大 -> Mitigation：按依赖拆分 tasks，每项使用独立 agent、定向测试和暂存，最终统一验证。
- [Risk] 严格解码会使遗漏的旧部署配置直接启动失败 -> Mitigation：对仓库配置、fixture、Compose、Kubernetes、Helm 和环境变量执行全仓扫描并增加旧字段失败测试。
- [Risk] pprof 迁移可能改变运维访问路径 -> Mitigation：保持 helper 可复用，提供明确环境变量、loopback 默认值和测试覆盖。
- [Risk] cache 关闭可能暴露隐含正确性依赖 -> Mitigation：为 auth 和 RBAC cache 增加 disabled 测试，确认只影响性能。

## Migration Plan

1. 完成并验证本 change 的 artifacts。
2. 按 tasks 顺序建立 resources、核心 Config、严格解码、datastore 和服务自有配置。
3. 迁移 providers、HTTP、cache、logging、pprof、trusted proxies 和 tracing。
4. 更新全部运行配置、部署资产、文档和规格；扫描并删除旧契约使用点。
5. 将全部预期变更暂存后运行 OpenSpec 校验、架构 lint、`make lint` 和 `make verify`。
6. 门禁全部通过后归档，并再次验证主规格和工作区无 drift。

回滚必须整体撤销本 change，不允许只恢复部分旧字段形成混合契约。

## Open Questions

无。
