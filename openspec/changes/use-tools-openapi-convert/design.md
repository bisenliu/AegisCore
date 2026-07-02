## Context

当前 OpenAPI 生成流程由 `user-service/scripts/openapi-generate.sh` 调用 `swag` 生成 Swagger 2 JSON，再调用 `user-service/internal/tools/openapi-convert/` 将其转换为 OpenAPI 3 并渲染 `user-service/docs/openapi.go`、`openapi.json` 和 `openapi.yaml`。转换核心已经位于 `common/http/openapi`，但 CLI 壳层仍放在 user-service 的 `internal/` 目录下，导致服务内业务边界与交付工具边界混在一起。

本变更只调整 OpenAPI 转换 CLI 的归属和调用方式，不改变 OpenAPI 文档内容、HTTP API 行为、数据库 schema、部署资产或 RBAC 授权语义。

## Goals / Non-Goals

**Goals:**

- 将 OpenAPI 转换 CLI 迁移到仓库级 `tools/openapi-convert/`，作为跨服务可复用的交付工具。
- 保持 `common/http/openapi` 作为转换、规范化、序列化和 Go embed 渲染的共享库边界。
- 让 `user-service/scripts/openapi-generate.sh` 显式传入 user-service 专属的 server、root server、探活路径、BearerAuth 文案和输出路径。
- 保持 `make user-service-openapi-generate` 和 `make verify` 的外部调用方式不变。

**Non-Goals:**

- 不新增或修改 HTTP endpoint。
- 不改变 OpenAPI 生成物的目标路径或运行时 OpenAPI 路由。
- 不引入 YAML/JSON 配置文件管理 OpenAPI 生成参数。
- 不把 `common/` 改造成通用工具集合。
- 不处理未来其他服务的 `swag` 扫描范围或生成脚本。

## Decisions

### Decision: 使用仓库级 `tools/openapi-convert/`

OpenAPI 转换 CLI MUST 位于 `tools/openapi-convert/`，而不是 `user-service/internal/tools/`、`user-service/cmd/tools/` 或 `common/tools/`。

选择该位置的原因是 CLI 属于仓库交付工具，不属于 user-service 业务代码，也不是 `common` 的运行时共享库 primitive。仓库级 `tools/` 可以让未来服务复用同一个转换入口，同时避免其他服务依赖 `user-service` 私有目录。

备选方案：

- `user-service/cmd/tools/openapi-convert/`：改动较小，但仍绑定 user-service，未来服务调用路径语义不合理。
- `common/tools/openapi-convert/`：技术可行，但会扩大 `common` 从共享库到共享工具集合的职责，后续 CLI 依赖也更容易污染 `common/go.mod`。
- `common/cmd/openapi-convert/`：同样会模糊 `common` 的库边界，不符合当前 `common/http/openapi` 只承载 helper 的约束。

### Decision: 服务差异由服务脚本显式传参

仓库级 CLI MUST 不写死 user-service 的 `/api/v1`、`/livez`、`/readyz`、`/startupz`、`BearerAuth` 等服务语义默认值。`user-service/scripts/openapi-generate.sh` MUST 显式传入这些参数。

这样可以支持未来服务拥有不同的探活接口、业务 server、鉴权 scheme 或输出位置，而不需要修改或复制转换工具。

备选方案是保留 CLI 默认值，但这会把 user-service 约定固化到仓库级工具中，与跨服务复用目标冲突。

### Decision: 保持转换核心在 `common/http/openapi`

`tools/openapi-convert` MUST 作为薄 CLI，仅解析 flag、调用 `common/http/openapi`、写出文件并返回进程状态。Swagger/OpenAPI 转换、规范化、序列化和 Go embed 渲染能力继续由 `common/http/openapi` 提供。

这种拆分保持依赖方向清晰：服务脚本调用仓库工具，仓库工具调用共享库，业务运行时代码不依赖工具包。

### Decision: 使用独立 Go module 加入 `go.work`

`tools/openapi-convert` SHOULD 作为独立 Go module 加入根 `go.work`，并依赖 `github.com/aegiscore/common v0.0.0`。这样工具依赖不会进入 `user-service/go.mod`，也不会把 CLI 专用依赖加入 `common/go.mod`。

备选方案是在仓库根创建统一 tools module，但当前只有一个工具，独立模块最小且路径清晰。若未来工具增多，可以再评估是否合并为 `tools/go.mod`。

## Risks / Trade-offs

- [Risk] 新增 `tools/openapi-convert` module 后，`go.work` 或 IDE 索引需要识别第三个模块。→ Mitigation: 将 `./tools/openapi-convert` 加入 `go.work`，并通过 `go test ./...` 在该目录验证工具可编译。
- [Risk] 去除 CLI 服务语义默认值后，脚本漏传参数可能导致生成文档 server 或 security scheme 变化。→ Mitigation: 在 `user-service/scripts/openapi-generate.sh` 显式传入现有参数，并运行 `make user-service-openapi-generate` 检查生成物 diff。
- [Risk] `tools/` 顶层目录引入新的仓库区域。→ Mitigation: 规格约束该目录用于仓库级交付工具，转换核心仍保留在 `common/http/openapi`。
- [Risk] 迁移过程中旧路径残留导致脚本仍调用 `user-service/internal/tools/openapi-convert`。→ Mitigation: 搜索 `internal/tools/openapi-convert` 和 `openapi-convert`，确保调用点更新。

## Migration Plan

1. 新建 `tools/openapi-convert` module，并迁移当前 CLI 源码。
2. 调整 CLI 默认值，移除 user-service 专属 server、探活路径和 BearerAuth 默认语义。
3. 更新 `go.work`，纳入 `./tools/openapi-convert`。
4. 更新 `user-service/scripts/openapi-generate.sh`，改为调用 `../tools/openapi-convert` 并显式传入 user-service 参数。
5. 删除 `user-service/internal/tools/openapi-convert/`。
6. 运行 `make user-service-openapi-generate`、`make user-service-architecture-lint` 和工具模块测试。

回滚时可以恢复 `user-service/internal/tools/openapi-convert/` 和脚本旧调用路径，并从 `go.work` 移除 `./tools/openapi-convert`。

## Open Questions

- 暂无。本变更先保持脚本显式传参，不引入服务级 OpenAPI 生成配置文件。
