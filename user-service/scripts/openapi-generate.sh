#!/usr/bin/env sh
set -eu

# 基于源码注解生成用户服务 OpenAPI 3 文档。
#
# 这是围绕第三方 swag CLI 和仓库 openapi-convert 工具的薄封装：脚本只固定扫描范围、
# 输出路径、server、认证方案和生成标记，避免开发者手写一长串容易漂移的参数。
#
# 用法：
#   make user-service-openapi-generate
#   cd user-service && ./scripts/openapi-generate.sh
#
# 生成产物：
#   user-service/docs/openapi.go
#   user-service/docs/openapi.json
#   user-service/docs/openapi.yaml
#
# 执行前提：
#   - 本机需要安装 go，并能访问 Go module 依赖源。
#   - 在 Go workspace 可用的仓库中运行，脚本会先切换到 user-service 目录。
#   - swag 通过 go run 拉取固定版本 github.com/swaggo/swag/cmd/swag@v1.16.6。
#   - tools/openapi-convert 必须可通过 ../tools/openapi-convert 运行。
#
# 行为：
#   - 只扫描 cmd、internal/router 和各 feature HTTP transport 中的 Swagger 注解。
#   - 先在临时目录生成 Swagger 2.0 JSON，再调用仓库工具转换为 OpenAPI 3 JSON/YAML/Go embed 文件。
#   - 为 /api/v1 配置业务 API server，同时保留 /livez、/readyz、/startupz 等根路径健康探针。
#   - 配置 BearerAuth 认证方案，供 OpenAPI UI 录入 Bearer token。
#
# 注意事项：
#   - 脚本会覆盖 docs/openapi.go、docs/openapi.json 和 docs/openapi.yaml；提交前应检查 diff。
#   - 修改路由、Swagger 注解、认证方案或健康探针路径后需要重新运行本脚本。
#   - macOS 下 go run swag 使用 external link mode，以规避本地链接器兼容性问题。

if ! command -v go >/dev/null 2>&1; then
  # swag 和 openapi-convert 都通过 go run 执行，缺少 Go 工具链时无需继续创建临时目录。
  echo "openapi-generate: required command not found: go" >&2
  exit 1
fi

cd "$(dirname "$0")/.."

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

if [ "$(go env GOOS)" = "darwin" ]; then
  # Darwin 环境下显式使用外部链接，降低 swag 依赖在本地链接阶段失败的概率。
  swag_go_run() {
    go run -ldflags='-linkmode=external' github.com/swaggo/swag/cmd/swag@v1.16.6 "$@"
  }
else
  swag_go_run() {
    go run github.com/swaggo/swag/cmd/swag@v1.16.6 "$@"
  }
fi

# swag 仅负责从源码注解生成 Swagger 2.0 中间文件；最终 OpenAPI 3 由 openapi-convert 统一规范化。
swag_go_run init \
  -d ./cmd,./internal/router,./internal/features/auth/transport/http,./internal/features/user/transport/http,./internal/features/role/transport/http,./internal/features/permission/transport/http \
  -g main.go \
  -o "$tmp_dir/swagger" \
  --useStructName \
  --parseDependency \
  --parseInternal

go run ../tools/openapi-convert \
  -input "$tmp_dir/swagger/swagger.json" \
  -json docs/openapi.json \
  -yaml docs/openapi.yaml \
  -go docs/openapi.go \
  -server /api/v1 \
  -root-server / \
  -root-path /livez \
  -root-path /readyz \
  -root-path /startupz \
  -bearer-auth-name BearerAuth \
  -bearer-auth-description "输入 Bearer token，格式为：Bearer <token>" \
  -generated-by scripts/openapi-generate.sh
