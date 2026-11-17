#!/usr/bin/env bash
# 按 project.yaml 替换仓库级项目身份。
# 不移动服务目录，不修改系统 RBAC UUID，不替换第三方依赖版本。
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
identity_file="${repo_root}/project.yaml"
apply=0
dry_run=1
skip_go_check=0
module_prefix=""
env_prefix=""
metric_prefix=""
image_prefix=""
display_name=""
changed_files=()

usage() {
  cat <<'EOF'
用法：
  ./scripts/rename-project.sh --dry-run \
    --module-prefix github.com/example \
    --env-prefix EXAMPLE \
    --metric-prefix example \
    --image-prefix example \
    --display-name Example

  ./scripts/rename-project.sh --apply \
    --module-prefix github.com/example \
    --env-prefix EXAMPLE \
    --metric-prefix example \
    --image-prefix example \
    --display-name Example

默认只预览。确认替换时必须追加 --apply。
脚本会先检查 go version。检查失败时不修改文件。
--apply 会更新 project.yaml、文本身份和 Go module 引用，然后执行 go work sync，
并重新生成 Ent 与 OpenAPI。生成失败时只恢复本次脚本写过的文件。
EOF
}

fail() {
  printf 'rename-project: %s\n' "$*" >&2
  exit 2
}

read_identity() {
  local key="$1"
  local value
  value="$(awk -F': ' -v key="${key}" '$1 == key {print $2; exit}' "${identity_file}")"
  if [[ -z "${value}" ]]; then
    fail "project.yaml 缺少 ${key}"
  fi
  printf '%s' "${value}"
}

validate_token() {
  local name="$1"
  local value="$2"
  local pattern="$3"
  if [[ ! "${value}" =~ ${pattern} ]]; then
    fail "${name} 格式不正确: ${value}"
  fi
}

require_go() {
  local version_output
  if ! version_output="$(go version 2>&1)"; then
    fail "本机 Go 工具链无法执行。请重新安装与 go.work 匹配的 Go，并确认 go version 成功后再运行 --apply。原始错误: ${version_output}"
  fi
  printf 'rename-project: %s\n' "${version_output}"
}

record_changed() {
  local file="$1"
  local existing
  for existing in "${changed_files[@]+"${changed_files[@]}"}"; do
    if [[ "${existing}" == "${file}" ]]; then
      return
    fi
  done
  changed_files+=("${file}")
}

replace_tree() {
  local from="$1"
  local to="$2"
  if [[ "${from}" == "${to}" ]]; then
    return
  fi
  local file
  local files=()
  while IFS= read -r -d '' file; do
    files+=("${file}")
  done < <(
    cd "${repo_root}"
    git grep -z -l -F -- "${from}" -- . \
      ':(exclude)user-service/internal/persistence/ent/**' \
      ':(exclude)user-service/internal/shared/rbacbaseline/ids.go' \
      ':(exclude)user-service/docs/openapi.go' \
      ':(exclude)user-service/docs/openapi.json' \
      ':(exclude)user-service/docs/openapi.yaml'
  )
  if [[ "${#files[@]}" -eq 0 ]]; then
    return
  fi
  printf 'replace %s -> %s in %d files\n' "${from}" "${to}" "${#files[@]}"
  if [[ "${apply}" -eq 1 ]]; then
    local changed
    for changed in "${files[@]}"; do
      record_changed "${changed}"
    done
    FROM="${from}" TO="${to}" perl -0pi -e 's/\Q$ENV{FROM}\E/$ENV{TO}/g' "${files[@]}"
  fi
}

write_identity() {
  record_changed "${identity_file}"
  cat > "${identity_file}" <<EOF
# 仓库级项目身份。新增根目录业务服务不改本文件。
module_prefix: ${module_prefix}
env_prefix: ${env_prefix}
metric_prefix: ${metric_prefix}
image_prefix: ${image_prefix}
display_name: ${display_name}
EOF
}

restore_changed_files() {
  local file relative
  for file in "${changed_files[@]+"${changed_files[@]}"}"; do
    relative="${file#"${repo_root}/"}"
    git -C "${repo_root}" checkout -- "${relative}" >/dev/null 2>&1 || true
  done
}

regenerate() {
  (
    cd "${repo_root}"
    go work sync
    # Ent 包内仍引用旧 module。先只生成 Ent，避免其他生成器加载损坏的 Ent 包。
    go -C user-service generate ./internal/persistence/ent
    make user-service-generate
    make user-service-openapi-generate
  )
}

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --dry-run) dry_run=1; apply=0 ;;
    --apply) dry_run=0; apply=1 ;;
    --skip-go-check) skip_go_check=1 ;;
    --module-prefix) module_prefix="${2:-}"; shift ;;
    --env-prefix) env_prefix="${2:-}"; shift ;;
    --metric-prefix) metric_prefix="${2:-}"; shift ;;
    --image-prefix) image_prefix="${2:-}"; shift ;;
    --display-name) display_name="${2:-}"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) fail "未知参数: $1" ;;
  esac
  shift
done

[[ -f "${identity_file}" ]] || fail "缺少 project.yaml"
current_module="$(read_identity module_prefix)"
current_env="$(read_identity env_prefix)"
current_metric="$(read_identity metric_prefix)"
current_image="$(read_identity image_prefix)"
current_display="$(read_identity display_name)"

[[ -n "${module_prefix}" && -n "${env_prefix}" && -n "${metric_prefix}" && -n "${image_prefix}" && -n "${display_name}" ]] || fail "必须同时提供五个身份参数"
validate_token "module_prefix" "${module_prefix}" '^github\.com/[A-Za-z0-9_.-]+$'
validate_token "env_prefix" "${env_prefix}" '^[A-Z][A-Z0-9_]*$'
validate_token "metric_prefix" "${metric_prefix}" '^[a-z][a-z0-9_]*$'
validate_token "image_prefix" "${image_prefix}" '^[a-z][a-z0-9-]*$'
validate_token "display_name" "${display_name}" '^[A-Za-z][A-Za-z0-9]*$'

if [[ "${dry_run}" -eq 1 && "${apply}" -eq 1 ]]; then
  fail "--dry-run 与 --apply 不能同时使用"
fi

if [[ "${skip_go_check}" -eq 1 ]]; then
  if [[ "${apply}" -eq 1 ]]; then
    fail "--apply 不能跳过 Go 工具链检查"
  fi
  printf 'rename-project: skipped go version check\n'
else
  require_go
fi

if [[ "${dry_run}" -eq 1 ]]; then
  printf 'rename-project: dry-run\n'
else
  printf 'rename-project: apply\n'
fi

if [[ "${apply}" -eq 1 ]]; then
  trap 'status=$?; printf "rename-project: 失败，正在恢复本次写过的文件\n" >&2; restore_changed_files; exit "${status}"' ERR
fi

replace_tree "${current_module}" "${module_prefix}"
replace_tree "${current_env}" "${env_prefix}"
replace_tree "${current_metric}_" "${metric_prefix}_"
replace_tree "${current_image}-" "${image_prefix}-"
replace_tree "${current_display}" "${display_name}"

if [[ "${apply}" -eq 1 ]]; then
  write_identity
  regenerate
  if ! git -C "${repo_root}" diff --exit-code -- user-service/internal/shared/rbacbaseline/ids.go >/dev/null; then
    fail "系统 RBAC ID 被修改，已停止"
  fi
  trap - ERR
fi

printf 'rename-project: ok\n'
