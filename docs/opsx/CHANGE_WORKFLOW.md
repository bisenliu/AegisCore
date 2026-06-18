# OPSX 变更工作流

## 先读基线

实施变更前先阅读：

- `AGENTS.md`
- `docs/ARCHITECTURE.md`
- `docs/opsx/CAPABILITY_MAP.md`
- 相关 `openspec/specs/*/spec.md`

这些文件描述当前有效边界。

所有新建或更新的 OpenSpec 主规格、change artifacts 和 OPSX 相关文档必须使用简体中文。包名、路径、HTTP method、配置 key、CLI 命令、Go symbol、错误码、数据库字段等技术标识符可以保留英文原文，但正文、标题、需求、场景、任务和面向协作者的说明不得保留英文模板内容。

## 命令

- `/opsx:explore`：讨论或澄清需求，定位受影响 capability 和代码区域。
- `/opsx:propose <change-name>`：为新需求创建 `proposal.md`、`design.md`、`tasks.md` 等 artifacts。
- `/opsx:apply <change-name>`：按 tasks 实施代码、文档和规格更新。
- `/opsx:verify <change-name>`：运行与变更范围匹配的验证，并检查规格与实现是否一致。
- `/opsx:archive <change-name>`：变更完成后，将稳定结果沉淀到 `openspec/specs/` 并归档 change。

## 规则

- 新 change 使用 kebab-case 名称。
- `/opsx:propose`、`/opsx:apply`、`/opsx:continue` 和 `/opsx:archive` 生成或更新的 `proposal.md`、`design.md`、`tasks.md`、`spec.md` 与相关文档必须使用简体中文。
- 跨 feature、跨模块、目录结构、外部契约、数据库 schema、RBAC、observability 或部署行为变更必须更新相关主规格。
- 实现完成后运行与范围匹配的 Makefile entrypoint；涉及 user-service OpenAPI 时运行 `make user-service-openapi-generate` 并检查 drift。
