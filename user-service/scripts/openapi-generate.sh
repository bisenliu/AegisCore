#!/usr/bin/env sh
set -eu

# 基于源码注解生成用户服务 OpenAPI 3 文档。
#
# 用法：
#   make openapi-generate
#   cd user-service && ./scripts/openapi-generate.sh
#   ./scripts/openapi-generate.sh
#
# 生成产物：
#   user-service/docs/openapi.go
#   user-service/docs/openapi.json
#   user-service/docs/openapi.yaml

cd "$(dirname "$0")/.."

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

if [ "$(go env GOOS)" = "darwin" ]; then
  swag_go_run() {
    go run -ldflags='-linkmode=external' github.com/swaggo/swag/cmd/swag@v1.16.6 "$@"
  }
else
  swag_go_run() {
    go run github.com/swaggo/swag/cmd/swag@v1.16.6 "$@"
  }
fi

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
