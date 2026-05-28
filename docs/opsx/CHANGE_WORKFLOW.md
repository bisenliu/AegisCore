# OPSX Change Workflow

本仓库使用 OPSX/OpenSpec 将需求探索、变更提案、实现任务和主规格归档分开处理。

## 1. Before A Change

1. 阅读 `AGENTS.md` 获取仓库导航。
2. 阅读 `docs/opsx/CAPABILITY_MAP.md` 定位相关 capability。
3. 阅读对应 `openspec/specs/<capability>/spec.md`，确认当前稳定行为。
4. 若需求仍不清楚，先运行 `/opsx:explore`。

## 2. Propose

使用 `/opsx:propose <change-name>` 创建 change artifacts。change name 使用 kebab-case，例如：

```text
/opsx:propose add-user-create-api
```

提案应说明：

- 为什么需要变更。
- 涉及哪些 capability。
- 是否新增长期能力或修改现有主规格。
- 非目标和兼容性风险。

## 3. Apply

准备实现时运行：

```text
/opsx:apply <change-name>
```

实现时遵循：

- 不直接编辑 Ent 生成代码。
- 保持 controller/service/repository 分层。
- HTTP 响应必须符合 `api-response-contract`。
- 涉及配置、基础设施或响应格式时同步更新相关主规格或 change specs。

## 4. Verify

本仓库的验证重点：

- Go 代码格式化：`gofmt`。
- 单元测试或集成测试：`go test ./...`。
- Ent schema 变更后：在 `user-services/` 执行 `go generate ./ent`。
- HTTP 行为变更后：验证成功响应、参数错误、not found 和 internal error。

## 5. Archive

实现完成并验证后，使用：

```text
/opsx:archive <change-name>
```

归档时应把已完成变更沉淀到 `openspec/specs/` 主规格中，确保 capability map 和主规格仍能反映当前系统事实。

## 6. When To Update Main Specs

- 新增稳定 API 或长期业务能力。
- 修改现有 API 响应、错误码或兼容行为。
- 修改服务启动、配置加载、中间件或基础设施生命周期。
- 修改用户模型约束或查询语义。

不要因为一次性重构或纯格式化变更更新主规格，除非外部可观察行为发生变化。
