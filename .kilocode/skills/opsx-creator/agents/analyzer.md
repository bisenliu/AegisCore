# OPSX 分析代理

你正在为仓库建立 OPSX / OpenSpec 基础设施做分析。

## 目标

产出足够准确的分析结果，供其他代理创建：
- AGENTS.md
- docs/ARCHITECTURE.md / docs/DEVELOPMENT.md / docs/PRODUCT.md / docs/TESTING.md
- docs/opsx/CAPABILITY_MAP.md / docs/opsx/CHANGE_WORKFLOW.md
- openspec/config.yaml
- openspec/specs/<capability>/spec.md

## 任务

### 1. 识别技术栈

优先检查：

```bash
ls package.json go.mod pyproject.toml requirements.txt Cargo.toml pom.xml build.gradle 2>/dev/null
```

记录：
- 语言 / 框架
- 包管理器
- 构建、测试、运行入口线索

### 2. 建立项目结构地图

扫描主要代码文件：

```bash
find . -type f \( -name "*.ts" -o -name "*.tsx" -o -name "*.js" -o -name "*.go" -o -name "*.py" -o -name "*.rs" -o -name "*.java" \) \
  ! -path './.git/*' ! -path './node_modules/*' ! -path './vendor/*' | head -200
```

识别：
- 入口目录
- 核心领域目录
- 基础设施目录
- 测试目录

### 3. 提取关键执行路径

至少追踪 3 条代表性路径：
- 主成功路径
- 关键异常路径
- 启动 / 初始化路径

格式示例：

```text
[src/index.ts:12] bootstrap()
    ↓
[src/server.ts:18] startServer()
    ↓
[src/modules/auth/service.ts:42] login()
```

### 4. 提取稳定能力（最关键）

从以下线索综合判断 capability：
- 路由名 / handler 名
- service / usecase / command 名称
- 数据模型或核心对象
- CLI 子命令
- README 和现有业务文档

为每个 capability 输出：
- `name`: kebab-case
- `summary`: 一句话能力描述
- `code_locations`: 相关文件或目录
- `entrypoints`: 入口点
- `inputs_outputs`: 关键输入输出
- `edge_cases`: 边界条件
- `existing_spec`: 是否已有 `openspec/specs/<capability>/spec.md`

### 5. 审计 OPSX 就绪度

检查是否存在并是否可信：
- `AGENTS.md`
- `docs/ARCHITECTURE.md`
- `docs/DEVELOPMENT.md`
- `docs/PRODUCT.md`
- `docs/opsx/CAPABILITY_MAP.md`
- `openspec/config.yaml`
- `openspec/specs/*/spec.md`

标记缺口：P0 / P1 / P2。

## 输出

写入：`openspec/.analysis/project.json`

推荐 JSON 结构：

```json
{
  "tech_stack": {
    "language": "Node.js",
    "package_manager": "npm"
  },
  "entrypoints": ["src/index.ts", "src/cli.ts"],
  "modules": [
    {"name": "auth", "paths": ["src/modules/auth"]}
  ],
  "capabilities": [
    {
      "name": "user-authentication",
      "summary": "处理用户登录与会话校验",
      "code_locations": ["src/modules/auth/service.ts"],
      "entrypoints": ["src/routes/auth.ts"],
      "inputs_outputs": ["email/password -> session token"],
      "edge_cases": ["密码错误", "用户不存在"],
      "existing_spec": false
    }
  ],
  "docs_readiness": {
    "agents_md": false,
    "architecture": false,
    "development": true,
    "product": false,
    "config": true,
    "main_specs": 0
  },
  "critical_flows": [
    {
      "name": "login",
      "flow": ["src/routes/auth.ts:10", "src/modules/auth/service.ts:42"]
    }
  ]
}
```

另外输出一份人类可读摘要到：`openspec/.analysis/project-summary.md`。