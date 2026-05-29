# OPSX Foundation Templates

用于搭建 OPSX 基础框架的模板集合。模板的作用是**统一结构**，不是鼓励空内容占位。

---

## 1. AGENTS.md 模板

```markdown
# <项目名> Agent Guide

本文件为 AI 代理提供仓库导航。

## 1. Quick Start

- [架构文档](docs/ARCHITECTURE.md)
- [开发说明](docs/DEVELOPMENT.md)
- [产品上下文](docs/PRODUCT.md)
- [能力地图](docs/opsx/CAPABILITY_MAP.md)
- [OPSX 工作流](docs/opsx/CHANGE_WORKFLOW.md)

## 2. 核心文档地图

| 文档 | 作用 |
|---|---|
| `docs/ARCHITECTURE.md` | 代码结构、模块边界、关键流程 |
| `docs/DEVELOPMENT.md` | 构建、测试、运行、调试 |
| `docs/PRODUCT.md` | 用户、目标、核心场景 |
| `docs/TESTING.md` | 验证方法与测试入口 |
| `docs/opsx/CAPABILITY_MAP.md` | capability 与代码/规格映射 |
| `docs/opsx/CHANGE_WORKFLOW.md` | 如何在本仓库使用 `/opsx:*` |

## 3. OpenSpec 基线

- `openspec/config.yaml`：仓库级规则
- `openspec/specs/<capability>/spec.md`：主规格基线
- `openspec/changes/`：具体 change artifacts

## 4. 推荐工作方式

1. 先看 capability map，确认改动属于哪个能力
2. 必要时先更新对应 main spec
3. 用 `/opsx:explore` 探索问题或方案
4. 用 `/opsx:propose <change-name>` 生成变更
5. 准备实现时再进入 `/opsx:apply`
```

---

## 2. docs/PRODUCT.md 模板

```markdown
# Product Context

## 1. 项目目标

这个项目解决什么问题、面向什么用户。

## 2. 用户与角色

| 角色 | 目标 | 关注点 |
|---|---|---|
| 用户A | ... | ... |

## 3. 核心场景

1. 场景一
2. 场景二
3. 场景三

## 4. 关键约束

- 性能
- 安全
- 兼容性
- 部署环境
```

---

## 3. docs/opsx/CAPABILITY_MAP.md 模板

```markdown
# Capability Map

| Capability | 业务说明 | 主要代码位置 | 主规格 | 状态 |
|---|---|---|---|---|
| `user-authentication` | 用户认证与会话管理 | `src/auth/` | `openspec/specs/user-authentication/spec.md` | ready |
| `signature-generation` | 生成签名摘要 | `utils.js` | `openspec/specs/signature-generation/spec.md` | ready |
| `mongo-connection` | MongoDB URL 解析与连接约束 | `utils.js` | `openspec/specs/mongo-connection/spec.md` | draft |
```

建议在表格后补充：
- 关键入口点
- 待补 capability
- 交叉依赖说明

---

## 4. spec.md 模板

```markdown
# <capability>

## Purpose

说明这个能力解决什么问题。

## Requirements

### Requirement: <requirement name>

系统必须……

#### Scenario: <happy path>
- **Given** ...
- **When** ...
- **Then** ...

#### Scenario: <error path>
- **Given** ...
- **When** ...
- **Then** ...

#### Scenario: <edge case>
- **Given** ...
- **When** ...
- **Then** ...
```

---

## 5. openspec/config.yaml 模板

```yaml
schema: spec-driven
context: |
  语言：中文（简体）
  所有产出物必须用简体中文撰写。
  技术栈: Node.js
rules:
  specs:
    - 使用 Given/When/Then 格式编写场景
    - 需求应优先对应稳定 capability，而不是临时实现细节
    - 覆盖主流程、异常流程和边界条件
  design:
    - 说明关键技术决策的理由与备选方案
    - 列出受影响文件和模块
    - 评估性能、安全、兼容性影响
  tasks:
    - 优先引用已存在的 skill / workflow
    - 任务应包含必要验证步骤
    - 每个任务保持 1-2 小时粒度
```