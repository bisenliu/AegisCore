---
name: opsx-creator
description: "Design and create OPSX/OpenSpec foundation for a repository: AGENTS.md, codebase documentation, capability maps, baseline main specs under openspec/specs, and repo-specific openspec/config.yaml. Creates framework and docs directly — never writes business/application code."
---

# OPSX Creator

为当前仓库搭建适合后续 `/opsx:*` 工作流的基础框架：**代码地图、产品上下文、开发说明、能力基线规格，以及 OpenSpec 配置**。

> **核心理念**：先减少未知，再加快变更。OPSX 并不只是“写 proposal / design / tasks”，它依赖一个**可被代理直接读取的项目底座**。如果仓库里没有清晰的产品上下文、代码结构说明和主规格（main specs），后续每个 change 都会重复做侦查工作。

> **职责边界**：本技能只创建 **框架、文档、配置、基线规格**；**绝不编写业务代码、接口实现、测试逻辑或功能修复**。

## 统一工作流

无论项目是空仓库、已有代码但没有 OpenSpec、还是已经部分使用 OPSX，这个技能都遵循同一个模式：

```text
┌────────────────────────────────────────────────────────────────────┐
│ Phase 1: 快速检测 + 意图确认                                       │
│ 当前项目是什么？已有多少 OPSX 基础？这次要补到什么程度？           │
└────────────────────────────────────────────────────────────────────┘
                                   ↓
┌────────────────────────────────────────────────────────────────────┐
│ Phase 2: 并行分析                                                  │
│ - 代码结构分析：模块、入口、依赖、关键流程                         │
│ - 产品/能力分析：用户、场景、能力边界                              │
│ - OPSX 就绪度分析：文档、spec、config 缺什么                       │
└────────────────────────────────────────────────────────────────────┘
                                   ↓
┌────────────────────────────────────────────────────────────────────┐
│ Phase 3: 差量综合                                                  │
│ 合并分析结果，得到要创建/更新的框架、文档与规格清单               │
└────────────────────────────────────────────────────────────────────┘
                                   ↓
┌────────────────────────────────────────────────────────────────────┐
│ Phase 4: 并行创建 / 更新                                           │
│ - 文档 Agent：AGENTS.md、docs/*、docs/opsx/*                      │
│ - 规格 Agent：openspec/specs/*                                     │
│ - 配置 Agent：openspec/config.yaml 与索引说明                      │
└────────────────────────────────────────────────────────────────────┘
                                   ↓
┌────────────────────────────────────────────────────────────────────┐
│ Phase 5: 验证 + 交接                                               │
│ 检查文件、场景格式、引用质量，并告知如何开始后续 /opsx 工作流      │
└────────────────────────────────────────────────────────────────────┘
```

---

## Phase 1：快速检测 + 意图确认

**目标**：在 5 分钟内判断当前仓库的技术栈、文档状态、OPSX 基础状态和用户期望范围。

### 1.1 快速检测

运行类似下面的扫描：

```bash
# 代码与文档基础
file_count=$(find . -type f ! -path './.git/*' ! -path './node_modules/*' ! -path './vendor/*' 2>/dev/null | wc -l)
code_files=$(find . -type f \( -name "*.go" -o -name "*.ts" -o -name "*.js" -o -name "*.py" -o -name "*.rs" -o -name "*.java" \) \
  ! -path './.git/*' ! -path './node_modules/*' ! -path './vendor/*' 2>/dev/null | wc -l)

# OPSX / OpenSpec 相关
has_agents_md=$(test -f AGENTS.md && echo "yes" || echo "no")
has_architecture=$(test -f docs/ARCHITECTURE.md && echo "yes" || echo "no")
has_development=$(test -f docs/DEVELOPMENT.md && echo "yes" || echo "no")
has_product=$(test -f docs/PRODUCT.md && echo "yes" || echo "no")
has_opsx_docs=$(test -f docs/opsx/CAPABILITY_MAP.md && echo "yes" || echo "no")
has_openspec=$(test -d openspec && echo "yes" || echo "no")
has_openspec_config=$(test -f openspec/config.yaml && echo "yes" || echo "no")
spec_count=$(find openspec/specs -name spec.md 2>/dev/null | wc -l | tr -d ' ')

# 技术栈探测
if test -f package.json; then TECH="TypeScript/Node.js"
elif test -f go.mod; then TECH="Go"
elif test -f pyproject.toml || test -f requirements.txt; then TECH="Python"
elif test -f Cargo.toml; then TECH="Rust"
elif test -f pom.xml || test -f build.gradle; then TECH="Java"
else TECH="Unknown"
fi
```

### 1.2 项目状态分类

根据扫描结果判断：

| 状态 | 判定标准 | 行动 |
|---|---|---|
| **空项目** | `code_files = 0` 且缺少 OpenSpec 基础 | 创建最小 OPSX 脚手架与意图型文档 |
| **代码项目 / 无 OPSX** | 有代码，但 `openspec/` 或关键 docs 缺失 | 做完整分析并创建基础框架 |
| **部分 OPSX** | 有 `openspec/`，但主规格、能力地图或文档缺失 | 做 gap analysis，只补缺口 |
| **OPSX 基础齐全** | docs、config、main specs 基本都有 | 审计并提出增量完善建议 |

### 1.3 范围确认

如果可用 AskUserQuestion，先确认范围：

```json
{
  "question": "你希望这次把仓库补到什么程度的 OPSX 基础？",
  "options": [
    "完整基础（推荐）：AGENTS.md + docs + docs/opsx + openspec/config.yaml + 主规格基线",
    "文档优先：先补 AGENTS.md、架构/开发/产品文档，规格后补",
    "规格优先：先补 capability map 和 openspec/specs，文档保持最小"
  ]
}
```

如果无法提问，则默认使用：**完整基础（推荐）**。

如果项目是空项目，还应额外确认：
- 技术栈
- 产品类型（CLI / API / Web / 库 / 服务）
- 初始核心能力（建议 2-5 个）

---

## Phase 2：并行分析

**目标**：理解真实代码结构与真实能力边界，而不是凭目录名猜测。

### 2.1 并行分析代理

优先并行生成三类分析（小项目可内联完成）：

```text
Agent("opsx-code-analyzer")
Agent("opsx-docs-and-readiness-auditor")
Agent("opsx-capability-analyzer")
```

#### A. 代码结构分析

需要回答：
1. 入口点在哪里（CLI、HTTP、worker、定时任务）
2. 主要模块/包/目录如何分层
3. 哪些是核心领域对象、服务、适配器
4. 关键执行路径是什么（至少 3 条）
5. 构建/测试/运行命令是什么

建议保存到：`openspec/.analysis/project.json`

#### B. 文档与就绪度审计

需要回答：
1. 已有哪些文档，是否可信、是否过时
2. 是否已有 `AGENTS.md`
3. 是否已有 `docs/ARCHITECTURE.md` / `docs/DEVELOPMENT.md`
4. `openspec/config.yaml` 是否存在且足够具体
5. `openspec/specs/` 是否为空，若非空是否与代码边界一致

建议保存到：`openspec/.analysis/readiness.json`

#### C. 能力分析

这是 OPSX 基础最重要的一步。要从代码和产品线索中抽出“长期稳定能力”，而不是临时实现细节。

能力识别来源包括：
- HTTP 路由 / RPC 接口 / CLI 子命令
- service/usecase/controller 名称
- 数据模型与聚合根
- README、现有文档、环境变量、数据库表名

每个能力至少要得到：
- capability 名称（kebab-case）
- 业务目的
- 对应代码位置
- 涉及的关键输入/输出
- 风险点和边界条件
- 是否已经存在 main spec

建议保存到：`openspec/.analysis/capabilities.json`

### 2.2 空项目的特殊处理

如果项目为空：
- 跳过代码结构分析中的“真实依赖”部分
- 改为围绕用户提供的项目类型推导文档骨架
- 只创建**意图清晰的基础文档与能力草图**，不虚构实现细节

---

## Phase 3：差量综合

**目标**：只创建真正有助于后续 `/opsx:*` 的底座，而不是堆文档。

根据分析结果形成清单：

```markdown
## Delta

### 要创建
- [ ] AGENTS.md
- [ ] docs/ARCHITECTURE.md
- [ ] docs/DEVELOPMENT.md
- [ ] docs/PRODUCT.md
- [ ] docs/TESTING.md
- [ ] docs/opsx/CAPABILITY_MAP.md
- [ ] docs/opsx/CHANGE_WORKFLOW.md
- [ ] openspec/config.yaml
- [ ] openspec/specs/<capability>/spec.md

### 要更新
- [ ] 现有 config.yaml 过于泛化，需要补语言/技术栈/规则
- [ ] 现有 spec 缺少 Given/When/Then 场景

### 已满足
- [x] openspec/ 目录已存在
- [x] docs/ 结构可复用
```

### 默认产出集合

除非用户明确要求缩减，否则优先创建 / 更新这些文件：

1. `AGENTS.md`
2. `docs/ARCHITECTURE.md`
3. `docs/DEVELOPMENT.md`
4. `docs/PRODUCT.md`
5. `docs/TESTING.md`
6. `docs/opsx/CAPABILITY_MAP.md`
7. `docs/opsx/CHANGE_WORKFLOW.md`
8. `openspec/config.yaml`
9. `openspec/specs/<capability>/spec.md`（核心能力基线）

**注意**：这些是“主规格基础设施”，不是 change artifacts。不要在这个技能里生成 `openspec/changes/<name>/...` 的实现型变更，除非用户明确要求。

---

## Phase 4：并行创建 / 更新

### 4.1 文档 Agent

负责创建 / 更新：
- `AGENTS.md`
- `docs/ARCHITECTURE.md`
- `docs/DEVELOPMENT.md`
- `docs/PRODUCT.md`
- `docs/TESTING.md`
- `docs/opsx/CAPABILITY_MAP.md`
- `docs/opsx/CHANGE_WORKFLOW.md`

要求：
- 所有判断尽量引用真实代码位置（`file:line`）
- 不要使用“以后补充”的空模板，内容必须可用
- `AGENTS.md` 作为导航图，控制在约 80-140 行
- `CHANGE_WORKFLOW.md` 必须结合本仓库说明如何使用 `/opsx:explore`、`/opsx:propose`、`/opsx:apply`、`/opsx:verify`、`/opsx:archive`
- `CAPABILITY_MAP.md` 必须把 **能力 → 代码模块 → 主规格** 串起来

### 4.2 规格 Agent

负责创建 / 更新：
- `openspec/specs/<capability>/spec.md`

要求：
- 每个 spec 描述**稳定能力**，不是某个 commit 的临时实现
- 使用 Given / When / Then 场景
- 必须覆盖主流程、异常流程、边界条件
- 如果项目很大，先覆盖 3-8 个核心能力，并在 `CAPABILITY_MAP.md` 标注剩余待补能力
- 若已有 spec，优先增量修复，不要推倒重写

### 4.3 配置 Agent

负责创建 / 更新：
- `openspec/config.yaml`
- 可选：`openspec/specs/README.md`（若 specs 很多时）

要求：
- 配置中明确：语言、输出语言、技术栈、设计规则、tasks 规则
- 规则要与仓库现实一致，不要写笼统口号
- 如果仓库已有明确约束（例如中文输出、Node.js、Given/When/Then），要保留并细化

### 4.4 推荐子代理提示骨架

如果支持子代理，优先并发：

```text
Agent("create-opsx-docs", prompt="基于分析结果创建 AGENTS.md、docs/*、docs/opsx/*，不写业务代码")
Agent("create-opsx-specs", prompt="基于 capability analysis 创建/修复 openspec/specs/*/spec.md，遵循 Given/When/Then")
Agent("create-opsx-config", prompt="基于仓库上下文更新 openspec/config.yaml，保证后续 /opsx 工作流可直接使用")
```

对于小项目（< 30 个代码文件），可直接内联执行，不必强行拆分子代理。

---

## Phase 5：验证 + 交接

### 5.1 验证清单

至少执行以下检查：

```bash
test -f AGENTS.md && echo "✓ AGENTS.md"
test -f docs/ARCHITECTURE.md && echo "✓ docs/ARCHITECTURE.md"
test -f docs/DEVELOPMENT.md && echo "✓ docs/DEVELOPMENT.md"
test -f docs/PRODUCT.md && echo "✓ docs/PRODUCT.md"
test -f docs/opsx/CAPABILITY_MAP.md && echo "✓ docs/opsx/CAPABILITY_MAP.md"
test -f docs/opsx/CHANGE_WORKFLOW.md && echo "✓ docs/opsx/CHANGE_WORKFLOW.md"
test -f openspec/config.yaml && echo "✓ openspec/config.yaml"

find openspec/specs -name spec.md 2>/dev/null | wc -l
grep -R "Given" openspec/specs 2>/dev/null >/dev/null && echo "✓ Given"
grep -R "When" openspec/specs 2>/dev/null >/dev/null && echo "✓ When"
grep -R "Then" openspec/specs 2>/dev/null >/dev/null && echo "✓ Then"
```

如果已安装 OpenSpec CLI，可额外执行：

```bash
openspec list --json >/dev/null && echo "✓ openspec CLI available"
```

### 5.2 交接摘要

最终交付应说明：
- 仓库当前技术栈
- 新建/更新了哪些 OPSX 基础文件
- 已建立哪些主规格能力基线
- 后续建议先执行什么（例如 `/opsx:explore`、`/opsx:propose`）

示例：

```markdown
## OPSX Foundation Ready

已完成：
- AGENTS.md 导航图
- 架构 / 开发 / 产品 / 测试文档
- 能力地图与 OPSX 工作流说明
- openspec/config.yaml 仓库规则
- 5 个核心能力的 main specs

接下来可以：
1. 用 `/opsx:explore` 讨论新需求
2. 用 `/opsx:propose <change-name>` 创建变更
3. 用 `/opsx:apply <change-name>` 开始实现
```

---

## 核心原则

### 1. 先建立“主语境”，再做 change

没有基线文档和主规格，后续每个 change 都会重复理解系统。

### 2. 能力优先，不要被实现细节绑架

`user-service` 不是能力；`账户管理`、`认证登录`、`配置解析` 才可能是能力。

### 3. 主规格必须稳定

`openspec/specs/*` 描述的是仓库当前长期成立的能力事实，而不是某个临时改造方案。

### 4. 文档要能支持代理直接行动

代理读完 `AGENTS.md` 和 `docs/opsx/CAPABILITY_MAP.md` 后，应能回答：
- 这个仓库是干什么的？
- 我该从哪里找相关代码？
- 要改哪个 capability 的 spec？
- 后续应该用哪个 `/opsx:*` 命令？

### 5. 不写业务代码

这个技能只搭建基础设施和说明文档。真正的功能开发应交给 `/opsx:apply` 或其他实现技能。

---

## 参考文件

| 文件 | 用途 |
|---|---|
| `references/foundation-templates.md` | AGENTS.md、docs、capability map、spec、config 模板 |
| `references/capability-extraction-guide.md` | 如何从代码中识别稳定能力 |
| `agents/analyzer.md` | 分析代理提示词 |
| `agents/creator-docs.md` | 文档创建代理提示词 |
| `agents/creator-specs.md` | 主规格创建代理提示词 |
| `agents/creator-config.md` | 配置创建代理提示词 |

如果项目很小，直接内联执行这些步骤即可；如果项目较大，优先使用并行子代理来提高质量和速度。