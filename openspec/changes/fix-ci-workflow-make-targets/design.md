## Context

仓库根 `Makefile` 通过服务前缀暴露 user-service 的私有交付目标，例如 `user-service-architecture-lint`、`user-service-openapi-generate` 和 `user-service-migrate-validate`。当前 `.github/workflows/ci.yml` 与 `.github/workflows/migrations.yml` 仍调用无服务上下文的 `architecture-lint`、`openapi-generate` 和 `migrate-validate`，这些目标在根 `Makefile` 中不存在，会导致 PR 校验和 migration 校验工作流失败。

本变更只修复 GitHub Actions 入口名称。它不改变 Go 代码、HTTP API、数据库 schema、OpenAPI 生成物、部署清单、观测资产或安全边界。

## Goals / Non-Goals

**Goals:**

- 让 CI verify job 调用根 `Makefile` 中存在的 user-service 架构 lint 和 OpenAPI 生成目标。
- 让 migrations workflow 调用根 `Makefile` 中存在的 user-service migration validate 目标。
- 保持 root `Makefile` 的服务前缀命名约束，避免新增无服务上下文的兼容别名。
- 用 OpenSpec delta 固化 CI 工作流必须引用存在的服务前缀目标这一交付运维规则。

**Non-Goals:**

- 不新增或重命名 root `Makefile` 目标。
- 不调整 `make verify` 的执行顺序和语义。
- 不修改 Ent schema、Atlas migration、OpenAPI 注解或生成物。
- 不改变 Docker、Compose、Kubernetes、Helm 或观测资产。

## Decisions

- 修改 GitHub Actions workflow，而不是在根 `Makefile` 增加 `architecture-lint`、`openapi-generate`、`migrate-validate` 兼容别名。这样符合仓库规则中“服务私有目标必须带服务名前缀”的约束，也避免未来多个服务出现同名目标时产生歧义。备选方案是新增别名，但这会弱化服务上下文边界。
- 在 `ci.yml` 中保留显式步骤 `make lint`、`make user-service-architecture-lint`、`make test`、`make user-service-openapi-generate` 和 `git diff --exit-code`，而不是直接替换为 `make verify`。这样保持现有工作流日志粒度和失败定位方式，只修复错误目标名。备选方案是使用 `make verify`，但它会改变 workflow 中显式命令结构。
- 在 `migrations.yml` 中只替换 validate 目标为 `make user-service-migrate-validate`。Atlas setup、触发条件和 job 边界保持不变，避免把本次 P0 修复扩大到 migration 发布策略调整。

## Risks / Trade-offs

- [Risk] workflow YAML 只能通过本地命令和静态检查部分验证，不能完全模拟 GitHub Actions runner 环境。→ Mitigation: 使用 `make -n user-service-architecture-lint user-service-openapi-generate user-service-migrate-validate` 验证根目标存在并能展开到对应 user-service 脚本，必要时再在 CI 中观察实际运行结果。
- [Risk] 只修复目标名可能遗漏其他 workflow 中同类错误。→ Mitigation: 搜索 `.github/workflows` 中的相关 `make` 调用，确认本次报告的三个目标全部替换。
- [Risk] 保留显式 workflow 命令会与未来 `make verify` 内容发生 drift。→ Mitigation: `delivery-operations` 规格新增 CI 入口要求，后续调整 verify 行为时同步检查 workflow 与 root Make 入口。

## Migration Plan

1. 修改 `.github/workflows/ci.yml` 和 `.github/workflows/migrations.yml` 中的目标名称。
2. 运行 `make -n user-service-architecture-lint user-service-openapi-generate user-service-migrate-validate` 验证根目标存在并展开正确。
3. 运行 `make user-service-architecture-lint` 验证 OpenSpec 和 OPSX 文档约束通过。
4. 如需回滚，恢复 workflow 中的旧目标名；该回滚只影响 CI 配置，不涉及数据库或运行时状态。

## Open Questions

- 无。
