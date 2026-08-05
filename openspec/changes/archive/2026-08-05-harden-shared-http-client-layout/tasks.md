## 1. 规格与设计

- [x] 1.1 创建 proposal、design 和 `shared-platform-primitives` spec delta，明确请求快照、浅拷贝、body/client 所有权和不变的外部集成边界
- [x] 1.2 运行 `openspec validate harden-shared-http-client-layout` 并确认 apply artifacts 完整

## 2. 重组共享 HTTP client

- [x] 2.1 将 package 契约迁入 `doc.go`，按类型、错误、默认值、构造、校验、client 选择和发送职责拆分同 package 文件，不改变导出符号
- [x] 2.2 在发送边界复制 query/form/header maps，裁剪 URL、method 与 proxy URL，并保持默认 timeout、TLS、cookie、retry、proxy、form/JSON 和错误语义
- [x] 2.3 明确 `SendRequest` 顺序复用、并发限制、body 引用和注入 Resty client 的调用方所有权

## 3. 示例与测试

- [x] 3.1 增加基本 JSON 请求、context 取消、`StatusError` 和注入 client 的 executable examples
- [x] 3.2 增加配置快照、首尾空白归一化、调用方 maps 不变和顺序复用测试
- [x] 3.3 对修改的 Go 文件执行 `gofmt`，运行 `cd common && go vet ./http/client`、`go test ./http/client` 和 `go test -race ./http/client`

## 4. 规格与最终质量门禁

- [x] 4.1 运行 `openspec validate harden-shared-http-client-layout`、`openspec list --specs`、`openspec validate --specs`、`make user-service-architecture-lint` 和 `git diff --check`
- [x] 4.2 检查 `git status --short`，只暂存本次 change 与 HTTP client 文件，复核 staged diff
- [x] 4.3 在预期变更已暂存后运行 `make lint`；只有命令成功时才能勾选本任务
- [x] 4.4 在预期变更已暂存后运行 `make verify`，确认生成物 drift 与最终 `git diff --exit-code` 门禁通过；只有命令成功时才能勾选本任务
