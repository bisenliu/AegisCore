# OPSX 变更工作流

本仓库使用 OpenSpec 的 `spec-driven` schema 管理跨 feature、跨模块、外部契约、schema、部署或行为变更。目标是先明确 capability 和影响，再实施。

## 1. 何时使用 OPSX

需要先走 OPSX 的情况：

- 改变用户、认证、RBAC、共享契约、OpenAPI、数据库 schema、部署或观测行为。
- 影响多个 feature、多个模块或 `common/` 共享能力。
- 新增、删除或修改长期稳定 capability。
- 需要迁移、回滚、发布顺序或安全影响分析。

可以直接改的情况：

- 小范围文案、注释或无行为变化的修正。
- 明确不影响主规格的局部测试补充。
- 已有 tasks 中的实现步骤。

## 2. 常用命令

探索需求：

```text
/opsx:explore <问题或需求>
```

创建 change：

```text
/opsx:propose <change-name>
```

继续补 artifact：

```text
/opsx:continue <change-name>
```

实施 tasks：

```text
/opsx:apply <change-name>
```

验证：

```text
/opsx:verify <change-name>
```

归档：

```text
/opsx:archive <change-name>
```

## 3. change 命名

- 使用 kebab-case，例如 `add-session-revocation`、`update-rbac-route-diff`。
- 使用动词加对象，避免只写 `fix`、`refactor` 或目录名。
- 名称应表达业务或平台目标，而不是临时实现方式。

## 4. artifacts

一个 spec-driven change 通常包含：

| 文件 | 作用 |
|---|---|
| `proposal.md` | 说明为什么做、做什么、涉及哪些 capability 和影响面 |
| `design.md` | 说明如何做、关键决策、备选方案、风险和验证方式 |
| `specs/<capability>/spec.md` | 描述本次对 capability 的规格 delta |
| `tasks.md` | 按依赖顺序拆分可执行实现和验证任务 |

OpenSpec 的 `context` 和 `rules` 是给代理使用的约束，不应复制到 artifact 正文。

## 5. proposal 最小模板

```markdown
## Why

用 1-2 段说明问题、机会和为什么现在要做。

## What Changes

- 具体说明新增、修改或移除的能力。
- 如果有 breaking change，明确标注。

## Capabilities

### New Capabilities

- `<capability-name>`: 说明新增能力覆盖什么。

### Modified Capabilities

- `<existing-capability>`: 说明哪个需求会变化。

## Impact

- 受影响代码、API、数据库、部署、观测、安全或文档。
```

## 6. design 最小模板

```markdown
## Context

说明当前状态、约束和相关路径。

## Goals / Non-Goals

**Goals:**

- 本次设计要达成什么。

**Non-Goals:**

- 明确不做什么。

## Decisions

### Decision: 决策名称

说明选择、理由和备选方案。

## Risks / Trade-offs

- [Risk] 风险描述 -> Mitigation：缓解方式。

## Migration Plan

说明实施、发布、回滚和验证步骤。
```

## 7. spec delta 最小模板

```markdown
## ADDED Requirements

### Requirement: 中文需求名称

系统 MUST 满足的稳定行为。

#### Scenario: 中文场景名称

- **WHEN** 触发条件
- **THEN** 预期结果
```

修改既有行为时使用 `## MODIFIED Requirements`，并复制完整 requirement 后再修改。移除行为时使用 `## REMOVED Requirements`，必须写明 Reason 和 Migration。

## 8. tasks 最小模板

```markdown
## 1. 任务组

- [ ] 1.1 创建或修改具体文件
- [ ] 1.2 运行具体验证命令

## 2. 验证

- [ ] 2.1 执行 `make user-service-architecture-lint`
```

tasks 必须使用 `- [ ]` checkbox，完成后立即改为 `- [x]`。

## 9. 实施前检查

运行 `/opsx:apply <change-name>` 前确认：

1. `openspec status --change <change-name>` 显示 apply 依赖已完成。
2. proposal、design、spec delta 和 tasks 内容一致。
3. capability 已在 `docs/opsx/CAPABILITY_MAP.md` 或本次 delta 中说明。
4. tasks 包含相关单元测试、暂存本次预期变更、`make lint` 和 `make verify` 验证命令。

## 10. 验证和归档

常用验证按以下顺序执行：

```bash
openspec validate <change-name>
openspec list --specs
openspec validate --specs
make user-service-architecture-lint
git add <本次预期变更文件>
make lint
make verify
```

`make lint` 和 `make verify` 必须在 OpenSpec 的实现、规格和文档任务全部完成后执行，并且执行前必须先暂存本次预期变更。`make verify` 的最终 `git diff --exit-code` 用于暴露生成物 drift 或未纳入暂存区的意外变更；如果未先暂存预期变更，验证会被正常实现 diff 阻塞，不能作为完成依据。

change 完成后：

1. 确认 tasks 全部为 `- [x]`。
2. 确认本次预期变更已暂存，且相关单元测试、`make lint` 和 `make verify` 全部通过；任一验证未通过或未运行时，不得将 OpenSpec 任务或 change 视为完成。
3. 运行 `/opsx:archive <change-name>`，把 delta specs 合并到 `openspec/specs/`。
4. 检查主规格、能力地图和 README 入口是否仍一致。
