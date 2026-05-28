## 1. 工具链版本确认

- [x] 1.1 确认可用安装源中的 Go 1.26 最新补丁版本，并记录要写入的 `toolchain go1.26.x` 精确值
- [x] 1.2 搜索仓库中的 CI 或工具链配置文件，确认是否存在除 `go.work` 和 `go.mod` 外需要同步 Go 版本的位置

## 2. 版本声明更新

- [x] 2.1 将根目录 `go.work` 的 `go` 声明更新为 `1.26`，并将 `toolchain` 更新为 Go 1.26 最新补丁版本
- [x] 2.2 将 `common/go.mod` 的 `go` 声明更新为 `1.26`，并按设计保持 toolchain 基线一致
- [x] 2.3 将 `user-services/go.mod` 的 `go` 声明更新为 `1.26`，并将 `toolchain` 更新为 Go 1.26 最新补丁版本
- [x] 2.4 如修改过程中触发 `go mod tidy` 变化，检查 `go.sum` 或依赖变更是否为 Go 1.26 兼容性所必需

## 3. 文档更新

- [x] 3.1 更新 `docs/DEVELOPMENT.md` 的 Go 前置条件，说明 workspace 使用 `go 1.26` 和 Go 1.26 最新补丁 toolchain
- [x] 3.2 确认文档中不再把 Go 1.23 或 Go 1.24 描述为当前工具链基线

## 4. 验证

- [x] 4.1 确认本变更未修改 Ent schema 或 `user-services/ent/` 生成代码，因此无需运行 `go generate ./ent`
- [x] 4.2 如修改了 Go 源码文件，对相关文件运行 `gofmt -w`；若仅修改 `go.mod`、`go.work` 和文档，则记录无需 gofmt
- [x] 4.3 在 `common/` 目录运行 `go test ./...` 并确认通过
- [x] 4.4 在 `user-services/` 目录运行 `go test ./...` 并确认通过
- [x] 4.5 运行 `openspec status --change "upgrade-go-to-1-26"`，确认 change 处于可应用状态
