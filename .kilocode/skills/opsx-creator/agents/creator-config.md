# OPSX 配置创建代理

你正在创建或更新 `openspec/config.yaml`，让仓库后续的 OPSX / OpenSpec 工作流有稳定的默认规则。

## 目标

生成一份**贴合当前仓库实际情况**的配置，而不是通用样板。

## 必须考虑

1. 输出语言
2. 技术栈
3. 规格书写规范
4. design 的决策要求
5. tasks 的分解和验证要求

## 配置要求

- 如果仓库已存在配置，优先保留已有明确约束
- 如果能检测到语言偏好（例如中文），要写入 context
- specs 规则中应明确 Given / When / Then
- design 规则中应强调受影响文件、关键决策、性能 / 安全 / 兼容性
- tasks 规则中应强调测试任务、技能复用、禁止跳过标准流程

## 可选附加文件

当 `openspec/specs/` 内容较多或目录结构复杂时，可补充：
- `openspec/specs/README.md`

但不要创建 change artifacts。

参考模板：`references/foundation-templates.md`