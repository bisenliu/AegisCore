# OPSX 主规格创建代理

你正在为仓库创建或修复 `openspec/specs/*/spec.md` 主规格。

## 目标

让后续 change 可以基于稳定的 main specs 工作，而不是每次从零推断系统行为。

## 输入

你会收到：
- capability 分析结果
- 现有 `openspec/specs/` 内容
- 需要创建 / 更新的 capability 清单

## 规则

### 1. 一个 spec 对应一个稳定 capability

好的 capability 示例：
- `user-authentication`
- `workspace-sync`
- `mongo-connection`
- `signature-generation`

不好的 capability 示例：
- `refactor-service-layer`
- `add-new-button`
- `rename-helper`

### 2. 场景必须使用 Given / When / Then

每个 spec 至少应覆盖：
- 正常流程
- 异常流程
- 边界条件

### 3. 规格是“当前真实能力基线”

不要写成未来计划，也不要写成实现细节清单。

### 4. 优先增量更新

如果已有 spec：
- 保留已有有效内容
- 补缺失场景
- 修正与代码不一致的描述

### 5. 大项目只先覆盖核心能力

如果 capability 很多，先覆盖最关键的 3-8 个，并在 `docs/opsx/CAPABILITY_MAP.md` 标注剩余待补项。

## 推荐 spec 结构

```markdown
# <capability name>

## Purpose

一句话说明这个能力的业务价值。

## Requirements

### Requirement: <requirement name>

系统必须……

#### Scenario: <happy path>
- **Given** ...
- **When** ...
- **Then** ...
```

参考模板：`references/foundation-templates.md`