#!/usr/bin/env bash
set -euo pipefail

# 验证 architecture-lint.sh 能发现关键架构违规，并且不会误报允许的测试或生成文件。
#
# 用法：
#   ./user-service/scripts/architecture-lint-test.sh
#
# 执行前提：
#   - 本机需要安装 architecture-lint.sh 依赖的 ripgrep（rg）。
#   - 脚本会在系统临时目录创建一个最小 Git fixture 仓库，结束时自动删除。
#
# 行为：
#   - 构造 common、user-service、tools/openapi-convert、CI workflow、deployments 和 openspec 的最小目录。
#   - 写入一组故意违规的 Go 文件和配置，用来覆盖架构分层、mock build tag、测试钩子、默认 logger、Fx/Dig 元数据等检查。
#   - 通过 --repo-root 让 architecture-lint.sh 针对 fixture 运行，而不是扫描真实仓库。
#   - 断言预期违规都会出现在输出中，并断言测试文件、生成目录、组合入口等白名单不会产生误报。
#
# 注意事项：
#   - 该脚本只验证 lint 规则的代表性路径，不替代真实仓库的 architecture-lint 扫描。
#   - 更新 architecture-lint.sh 的规则、报错文本或排除列表时，应同步更新这里的 fixture 和断言。
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fixture_root="$(mktemp -d)"
trap 'rm -rf "${fixture_root}"' EXIT

# 构造最小仓库布局；目录只包含触发或排除规则所需的文件。
mkdir -p \
  "${fixture_root}/.github/workflows" \
  "${fixture_root}/common" \
  "${fixture_root}/common/runtime/observability/metrics" \
  "${fixture_root}/common/runtime/example" \
  "${fixture_root}/common/testing/example" \
  "${fixture_root}/deployments/docker" \
  "${fixture_root}/deployments/compose" \
  "${fixture_root}/deployments/helm/aegiscore-user-service/templates" \
  "${fixture_root}/docs/opsx" \
  "${fixture_root}/openspec/changes" \
  "${fixture_root}/openspec/specs" \
  "${fixture_root}/tools/openapi-convert" \
  "${fixture_root}/user-service/ent" \
  "${fixture_root}/user-service/docs" \
  "${fixture_root}/user-service/internal/persistence/ent/schema" \
  "${fixture_root}/user-service/internal/features/auth/application" \
  "${fixture_root}/user-service/internal/features/auth/infrastructure" \
  "${fixture_root}/user-service/internal/features/role" \
  "${fixture_root}/user-service/internal/features/role/infrastructure" \
  "${fixture_root}/user-service/internal/features/user" \
  "${fixture_root}/user-service/internal/features/user/infrastructure/postgres" \
  "${fixture_root}/user-service/internal/features/user/transport/http" \
  "${fixture_root}/user-service/internal/shared" \
  "${fixture_root}/user-service/migrations" \
  "${fixture_root}/user-service/scripts"

cat > "${fixture_root}/common/go.mod" <<'EOF'
module github.com/aegiscore/common

go 1.26.5
EOF

cat > "${fixture_root}/go.work" <<'EOF'
go 1.26.5

toolchain go1.26.5

use (
	./common
	./tools/openapi-convert
	./user-service
)
EOF

cat > "${fixture_root}/tools/openapi-convert/go.mod" <<'EOF'
module github.com/aegiscore/tools/openapi-convert

go 1.26.5
EOF

cat > "${fixture_root}/user-service/go.mod" <<'EOF'
module github.com/aegiscore/user-service

go 1.26.5
EOF

cat > "${fixture_root}/.github/workflows/ci.yml" <<'EOF'
name: ci
on:
  pull_request:
jobs:
  quality:
    uses: ./.github/workflows/lint.yml
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@v7
        with:
          go-version: '1.26.5'
      - run: |
          make lint
          make test
  container-test:
    runs-on: ubuntu-latest
    steps:
      - run: make -C user-service test-containers
EOF

cat > "${fixture_root}/.github/workflows/lint.yml" <<'EOF'
name: quality
on:
  workflow_call:
  pull_request:
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@v7
        with:
          go-version: '1.26.5'
      - run: make lint
  unit:
    runs-on: ubuntu-latest
    steps:
      - run: make test
EOF

cat > "${fixture_root}/deployments/docker/atlas-postgres-pgtrgm.Dockerfile" <<'EOF'
FROM postgres:latest
EOF

cat > "${fixture_root}/user-service/scripts/migrate-diff.sh" <<'EOF'
ATLAS_DEV_IMAGE="aegiscore-atlas-postgres-pgtrgm:latest"
EOF

cat > "${fixture_root}/user-service/migrations/atlas.hcl" <<'EOF'
  default = "docker+postgres://_/aegiscore-atlas-postgres-pgtrgm:latest/dev?search_path=public"
EOF

cat > "${fixture_root}/deployments/compose/docker-compose.yml" <<'EOF'
services:
  postgres:
    image: postgres:latest
  user-service:
    image: aegiscore/user-service:local
    command:
      - serve
      - --config
EOF

cat > "${fixture_root}/deployments/helm/aegiscore-user-service/values.yaml" <<'EOF'
image:
  repository: aegiscore-user-service
  tag: latest
  pullPolicy: IfNotPresent
EOF

cat > "${fixture_root}/deployments/helm/aegiscore-user-service/values-local.yaml" <<'EOF'
image:
  ref: aegiscore-user-service:latest
EOF

cat > "${fixture_root}/deployments/helm/aegiscore-user-service/Chart.yaml" <<'EOF'
apiVersion: v2
name: aegiscore-user-service
type: application
version: 0.1.0
appVersion: "latest"
EOF

cat > "${fixture_root}/deployments/helm/aegiscore-user-service/templates/_helpers.tpl" <<'EOF'
{{- define "aegiscore-user-service.image" -}}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) -}}
{{- end -}}
EOF

cat > "${fixture_root}/deployments/helm/aegiscore-user-service/templates/deployment.yaml" <<'EOF'
image: {{ .Values.image.ref | quote }}
EOF

cat > "${fixture_root}/deployments/helm/aegiscore-user-service/templates/rbac-seed-job.yaml" <<'EOF'
image: {{ .Values.image.ref | quote }}
EOF

cat > "${fixture_root}/user-service/internal/features/auth/application/violating_import.go" <<'EOF'
package application

import _ "github.com/aegiscore/user-service/internal/features/auth/transport/http"
EOF

cat > "${fixture_root}/common/runtime/example/mock_generate.go" <<'EOF'
package example

//go:generate go tool mockgen -destination=mock_test.go -package=example example.org/project Port
EOF

cat > "${fixture_root}/common/runtime/example/root_ent_import.go" <<'EOF'
package example

import _ "github.com/aegiscore/user-service/ent"
EOF

cat > "${fixture_root}/common/runtime/observability/metrics/casbin_reload.go" <<'EOF'
package metrics

const casbinReloadsMetricName = "aegiscore_casbin_policy_reloads_total"
EOF

cat > "${fixture_root}/user-service/internal/features/auth/application/test_hook.go" <<'EOF'
package application

func setClockForTest() {}
EOF

cat > "${fixture_root}/user-service/internal/features/auth/application/fx_metadata.go" <<'EOF'
package application

import "go.uber.org/fx"

type Deps struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

cat > "${fixture_root}/user-service/internal/features/role/fx.go" <<'EOF'
package role

import "go.uber.org/fx"

type Params struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

cat > "${fixture_root}/user-service/internal/features/role/infrastructure/store.go" <<'EOF'
package infrastructure

import "go.uber.org/fx"

type Params struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

cat > "${fixture_root}/user-service/internal/features/user/infrastructure/postgres/fx_metadata.go" <<'EOF'
package postgres

import "go.uber.org/fx"

type Params struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

cat > "${fixture_root}/user-service/internal/features/user/fx.go" <<'EOF'
package user

import "go.uber.org/fx"

type Params struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

cat > "${fixture_root}/user-service/internal/features/user/route_registrar.go" <<'EOF'
package user
EOF

cat > "${fixture_root}/user-service/internal/features/user/infrastructure/postgres/store_test.go" <<'EOF'
package postgres

import "go.uber.org/fx"

type TestParams struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

cat > "${fixture_root}/user-service/internal/features/user/transport/http/mock_generate.go" <<'EOF'
//go:build generate

package http

import "go.uber.org/fx"

type Params struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

cat > "${fixture_root}/user-service/internal/features/auth/infrastructure/default_logger.go" <<'EOF'
package infrastructure

import (
	"context"

	"github.com/aegiscore/common/runtime/logger"
)

func useDefaultLogger() {
	logger.Info(context.Background(), "default logger dependency")
}
EOF

cat > "${fixture_root}/user-service/internal/features/auth/infrastructure/env_config.go" <<'EOF'
package infrastructure

import "os"

func loadEnvConfig() string {
	return os.Getenv("USER_SERVICE_CONFIG")
}
EOF

cat > "${fixture_root}/user-service/internal/features/auth/application/allowed_test.go" <<'EOF'
package application

func setRequestForTest() {}
EOF

cat > "${fixture_root}/common/testing/example/helper.go" <<'EOF'
package example

func NewStoreForTest() {}
EOF

cat > "${fixture_root}/user-service/internal/persistence/ent/schema/generated.go" <<'EOF'
package schema

func testHookGenerated() {}
EOF

cat > "${fixture_root}/user-service/docs/openapi.go" <<'EOF'
package docs

func NewDocumentForTest() {}
EOF

git -C "${fixture_root}" init -q

# 将 lint 根目录指向 fixture，避免测试依赖真实工作区的当前状态。
set +e
output="$("${script_dir}/architecture-lint.sh" --repo-root "${fixture_root}" 2>&1)"
status=$?
set -e

if [[ "${status}" -eq 0 ]]; then
  printf 'architecture-lint-test: expected fixture violation to fail\n%s\n' "${output}" >&2
  exit 1
fi

# 下列断言确保每类核心规则至少有一个 fixture 能触发对应失败输出。
if [[ "${output}" != *"application/domain/infrastructure must not import feature HTTP transport DTO/controller packages"* ]]; then
  printf 'architecture-lint-test: expected layering violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"mock_generate.go must use //go:build generate"* ]]; then
  printf 'architecture-lint-test: expected mock generate build tag violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"test-only symbol must not enter production Go files"* ]]; then
  printf 'architecture-lint-test: expected test-only production symbol violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"feature production code must not use package-level default logger as main-path dependency"* ]]; then
  printf 'architecture-lint-test: expected default logger dependency violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"production Go code must not retain local config path entrypoints"* ]]; then
  printf 'architecture-lint-test: expected Go local config path violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"root-level user-service/ent package is forbidden"* ]]; then
  printf 'architecture-lint-test: expected root-level Ent directory violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"root-level user-service Ent import is forbidden"* ]]; then
  printf 'architecture-lint-test: expected root-level Ent import violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"Docker Compose runtime config must not mount local full config or pass --config"* ]]; then
  printf 'architecture-lint-test: expected Compose local config path violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"Helm production values must require explicit image.ref"* ]]; then
  printf 'architecture-lint-test: expected Helm image.ref required violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"Helm image helper must fail on latest image ref"* ]]; then
  printf 'architecture-lint-test: expected Helm latest guard violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"feature application/domain production code must not carry Fx DI metadata"* ]]; then
  printf 'architecture-lint-test: expected application/domain Fx metadata violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"common runtime metrics must not contain service or RBAC business metrics semantics"* ]]; then
  printf 'architecture-lint-test: expected common metrics business semantics violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"quality workflow must not directly trigger pull_request or push"* || "${output}" != *"CI standard lint command must appear exactly once"* || "${output}" != *"CI standard unit test command must appear exactly once"* || "${output}" != *"CI Docker-backed test command must call root make test-containers exactly once"* || "${output}" != *"CI must not bypass root make test-containers with module-local container targets"* ]]; then
  printf 'architecture-lint-test: expected duplicate CI quality gate violation reports\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"user feature production code must not carry Fx/Dig DI metadata outside composition"* ]]; then
  printf 'architecture-lint-test: expected user feature Fx/Dig metadata violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"role feature production code must not carry Fx/Dig DI metadata outside composition"* ]]; then
  printf 'architecture-lint-test: expected role feature Fx/Dig metadata violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"fixed feature route_registrar.go files are forbidden"* ]]; then
  printf 'architecture-lint-test: expected fixed feature route registrar violation report\n%s\n' "${output}" >&2
  exit 1
fi

# 白名单文件不应进入违规结果；这里覆盖测试文件、测试 helper、Ent/OpenAPI 生成目录和 feature 组合入口。
if [[ "${output}" == *"allowed_test.go"* || "${output}" == *"common/testing/example/helper.go"* || "${output}" == *"user-service/internal/persistence/ent/schema/generated.go"* || "${output}" == *"user-service/docs/openapi.go"* || "${output}" == *"user-service/internal/features/role/fx.go"* || "${output}" == *"user-service/internal/features/user/fx.go"* || "${output}" == *"user-service/internal/features/user/infrastructure/postgres/store_test.go"* || "${output}" == *"user-service/internal/features/user/transport/http/mock_generate.go"* ]]; then
  printf 'architecture-lint-test: excluded test or generated file produced a false positive\n%s\n' "${output}" >&2
  exit 1
fi

# fixture 内的路径都应存在；如果出现 rg 执行错误，通常意味着规则路径或测试目录布局需要同步调整。
if [[ "${output}" == *"rg execution failed"* ]]; then
  printf 'architecture-lint-test: unexpected rg execution failure\n%s\n' "${output}" >&2
  exit 1
fi

printf 'architecture-lint-test: ok\n'
