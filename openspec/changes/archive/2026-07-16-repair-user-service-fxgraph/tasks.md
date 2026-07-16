## 1. 现状确认

- [x] 1.1 定位 `user-service/cmd` 中现有 `fxgraph` 命令、测试、Makefile 目标和受版本控制的 Fx dependency graph 资产。
- [x] 1.2 复现 `cd user-service && go run ./cmd fxgraph --config ./configs/config.yaml --output /tmp/aegis-fx.dot` 的当前失败，并确认缺失 `*serviceconfig.Config` 等关键依赖。
- [x] 1.3 确认 `unify-user-service-app-configuration` 提供的正式 App 基础 input/options builder 入口及其依赖边界。

## 2. fxgraph 装配修复

- [x] 2.1 调整 `user-service/cmd` 的 `fxgraph` 组装逻辑，复用正式 App 基础 input/options builder，而不是维护独立 mock option 清单。
- [x] 2.2 在 user-service 命令层提供 `*serviceconfig.Config`、派生 runtime config、logger 和无外部副作用的 PostgreSQL、Redis、OTLP 或等价资源替身。
- [x] 2.3 确保 `fxgraph` 图生成不连接真实 PostgreSQL、Redis、OTLP，不启动 HTTP server，并且不改变正式 `serve` App 的业务行为。
- [x] 2.4 保持 `common/runtime/fxgraph` 只负责业务中立 DOT rendering，不引入 user-service 配置、feature provider、Ent 或外部资源依赖。

## 3. 测试与图资产

- [x] 3.1 将 `user-service/cmd` 中仅断言 option 数量的 fxgraph 测试替换或补充为实际调用 `common/runtime/fxgraph` DOT renderer 的 smoke test。
- [x] 3.2 在测试中断言 DOT 输出非空，并包含 AppModule 或等价顶层 module、auth、user、role、permission 等关键 feature 节点或依赖边。
- [x] 3.3 在测试中覆盖缺少 `*serviceconfig.Config` 等关键 App 输入时渲染失败的路径。
- [x] 3.4 重新生成或更新版本控制中的 user-service Fx dependency graph 资产。
- [x] 3.5 复用或补充带 `user-service-` 前缀的 fxgraph generate/check 目标，并确保 drift 检查覆盖图资产。

## 4. 验证

- [x] 4.1 执行 `cd user-service && go run ./cmd fxgraph --config ./configs/config.yaml --output /tmp/aegis-fx.dot`，确认命令成功且输出非空并包含关键节点。
- [x] 4.2 执行 `cd user-service && go test ./cmd -count=1`。
- [x] 4.3 执行现有或新增的 user-service fxgraph check/generate 目标，并用 `git diff --exit-code` 或目标自身 check 模式确认生成物无 drift。
- [x] 4.4 执行 `openspec validate repair-user-service-fxgraph`。
- [x] 4.5 执行 `make user-service-architecture-lint`。
- [x] 4.6 暂存本次预期代码、规格和生成物变更后执行 `make lint`。
- [x] 4.7 在暂存本次预期变更后执行 `make verify`，确认最终 drift 检查通过。
