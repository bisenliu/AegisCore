# Capability Extraction Guide

OPSX 的质量很大程度取决于你是否把代码正确抽象成“能力（capability）”。

## 1. 什么是 capability

capability 是一个**对外可理解、相对稳定、能长期存在**的系统能力。

它通常满足：
- 可以用业务语言描述
- 会跨越多个 change 持续存在
- 有明确输入 / 输出 / 约束
- 能被一个或多个主规格场景覆盖

## 2. 从哪里找 capability

### A. 接口与入口

- HTTP 路由
- RPC / GraphQL resolver
- CLI 子命令
- 定时任务 / worker 入口

这些入口常对应“用户想完成什么事”。

### B. 领域服务

典型命名：
- `authService`
- `workspaceManager`
- `paymentUseCase`
- `parseMgoUrl`

如果一个服务承载了完整业务目标，很可能就是 capability 候选。

### C. 核心对象

如果项目围绕某些核心对象运转，例如：
- user
- workspace
- change
- spec
- session

可以进一步问：围绕这个对象，系统真正提供了哪些长期能力？

## 3. 不要把这些当 capability

- “重构 service 层”
- “改成 async/await”
- “新增按钮颜色”
- “迁移到新的目录结构”

这些是实现任务或 change，不是 capability。

## 4. 命名建议

用 kebab-case，且尽量贴近业务语义：

- `user-authentication`
- `change-proposal-management`
- `mongo-url-parsing`
- `checksum-signature-generation`

避免：

- `auth-service`
- `button-handler`
- `refactor-auth`

## 5. 如何验证一个 capability 是否合理

问这 5 个问题：

1. 用户或系统能否明确感知这个能力？
2. 它是否可能在未来多个 change 中持续存在？
3. 是否能写出 3 个以上 Given/When/Then 场景？
4. 是否能定位到一组相对集中的代码？
5. 它是否独立于某次临时实现方案？

如果大多数答案是“否”，那它可能不是好 capability。

## 6. 输出建议

当你识别出 capability 时，至少记录：

```json
{
  "name": "mongo-url-parsing",
  "summary": "将带特殊字符密码的 MongoDB URL 转换为可安全连接的格式",
  "code_locations": ["utils.js:3-12"],
  "entrypoints": ["utils.js:3-12"],
  "inputs_outputs": ["mongoUrl -> escaped mongoUrl"],
  "edge_cases": ["空输入", "密码中包含特殊字符", "非标准 URL"],
  "existing_spec": false
}
```