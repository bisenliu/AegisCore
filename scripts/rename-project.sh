#!/usr/bin/env bash
# 按 project.yaml 替换仓库级项目身份。
# 不移动服务目录，不修改系统 RBAC UUID，不替换第三方依赖版本。
# .github 是 CI，只替换其中的项目身份，不删除 workflow。
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
identity_file="${repo_root}/project.yaml"
apply=0
dry_run=1
skip_go_check=0
skip_generate=0
module_prefix=""
env_prefix=""
metric_prefix=""
image_prefix=""
display_name=""
from_module=""
from_env=""
from_metric=""
from_image=""
from_display=""
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

project.yaml 已是新身份、仓库里仍有旧名字时，用 --from-* 指定旧身份再跑一次：

  ./scripts/rename-project.sh --apply \
    --from-module-prefix github.com/old \
    --from-env-prefix OLD \
    --from-metric-prefix old \
    --from-image-prefix old \
    --from-display-name Old \
    --module-prefix github.com/example \
    --env-prefix EXAMPLE \
    --metric-prefix example \
    --image-prefix example \
    --display-name Example

默认只预览。确认替换时必须追加 --apply。
脚本默认先检查 go version。检查失败时不修改文件。
--apply 会更新 project.yaml、文本身份和 Go module 引用，然后执行 go work sync，
并重新生成 Ent 与 OpenAPI。生成失败时只恢复本次脚本写过的文件。
本机 Go 无法执行、只需要补清文本时使用 --skip-generate。
此时不跑 go work sync、Ent 和 OpenAPI 生成，也不因生成失败回滚已替换的文件。

替换顺序：
  module path、环境变量前缀、指标前缀（含 testcontainers flag）、
  镜像前缀（含镜像仓库路径）、展示名，最后替换裸项目 slug。
裸 slug 使用 metric 前缀，覆盖 Makefile、测试、数据库名、namespace 和域名标签。
Helm chart 目录和 Grafana provisioning 文件名会随镜像前缀改名。
不删除 .github。
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

# 文本替换不碰 Ent 生成代码、系统 RBAC ID 和 OpenAPI 生成物。
# schema 下的手写测试仍要替换，否则旧 module import 会留在测试里。
replace_tree() {
  local from="$1"
  local to="$2"
  if [[ "${from}" == "${to}" ]]; then
    return
  fi
  local file
  local files=()
  while IFS= read -r -d '' file; do
    # 生成代码由后续 Ent / OpenAPI 生成覆盖。schema 下的手写测试必须替换，不能整目录跳过。
    case "${file}" in
      user-service/internal/persistence/ent/schema/*) ;;
      user-service/internal/persistence/ent/*) continue ;;
      user-service/internal/shared/rbacbaseline/ids.go) continue ;;
      user-service/docs/openapi.go|user-service/docs/openapi.json|user-service/docs/openapi.yaml) continue ;;
    esac
    files+=("${file}")
  done < <(
    cd "${repo_root}"
    git grep -z -l -F -- "${from}" -- .
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

rename_matching_paths() {
  local from="$1"
  local to="$2"
  if [[ "${from}" == "${to}" ]]; then
    return
  fi
  local path relative target parent
  local paths=()
  # 不进入 .git，也不把上次失败留下的空目录当成要改名的源。
  while IFS= read -r -d '' path; do
    if [[ -d "${path}" ]] && [[ -z "$(find "${path}" -mindepth 1 -print -quit)" ]]; then
      continue
    fi
    paths+=("${path}")
  done < <(find "${repo_root}" -name ".git" -prune -o -name "*${from}*" -print0)
  if [[ "${#paths[@]}" -eq 0 ]]; then
    return
  fi
  # 先改深层路径，避免父目录改名后子路径失效。
  local sorted=()
  while IFS= read -r path; do
    [[ -z "${path}" ]] && continue
    sorted+=("${path}")
  done < <(printf '%s\n' "${paths[@]}" | awk '{ print length, $0 }' | sort -nr | cut -d' ' -f2-)
  for path in "${sorted[@]}"; do
    [[ -e "${path}" ]] || continue
    relative="${path#"${repo_root}/"}"
    target="${relative//${from}/${to}}"
    if [[ "${relative}" == "${target}" ]]; then
      continue
    fi
    # 目标已存在时不能 git mv，否则会把源目录嵌套进目标目录。
    if [[ -e "${repo_root}/${target}" ]]; then
      printf 'skip rename %s -> %s (target exists)\n' "${relative}" "${target}"
      continue
    fi
    printf 'rename %s -> %s\n' "${relative}" "${target}"
    if [[ "${apply}" -eq 1 ]]; then
      parent="$(dirname "${target}")"
      if [[ "${parent}" != "." && ! -d "${repo_root}/${parent}" ]]; then
        mkdir -p "${repo_root}/${parent}"
      fi
      git -C "${repo_root}" mv "${relative}" "${target}"
    fi
  done
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
    --skip-generate) skip_generate=1 ;;
    --module-prefix) module_prefix="${2:-}"; shift ;;
    --env-prefix) env_prefix="${2:-}"; shift ;;
    --metric-prefix) metric_prefix="${2:-}"; shift ;;
    --image-prefix) image_prefix="${2:-}"; shift ;;
    --display-name) display_name="${2:-}"; shift ;;
    --from-module-prefix) from_module="${2:-}"; shift ;;
    --from-env-prefix) from_env="${2:-}"; shift ;;
    --from-metric-prefix) from_metric="${2:-}"; shift ;;
    --from-image-prefix) from_image="${2:-}"; shift ;;
    --from-display-name) from_display="${2:-}"; shift ;;
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
# 默认从 project.yaml 的当前身份替换。补清残留时用 --from-* 指向旧身份。
if [[ -n "${from_module}" || -n "${from_env}" || -n "${from_metric}" || -n "${from_image}" || -n "${from_display}" ]]; then
  [[ -n "${from_module}" && -n "${from_env}" && -n "${from_metric}" && -n "${from_image}" && -n "${from_display}" ]] || fail "--from-* 必须五个一起提供"
  current_module="${from_module}"
  current_env="${from_env}"
  current_metric="${from_metric}"
  current_image="${from_image}"
  current_display="${from_display}"
fi

[[ -n "${module_prefix}" && -n "${env_prefix}" && -n "${metric_prefix}" && -n "${image_prefix}" && -n "${display_name}" ]] || fail "必须同时提供五个身份参数"
validate_token "module_prefix" "${module_prefix}" '^github\.com/[A-Za-z0-9_.-]+$'
validate_token "env_prefix" "${env_prefix}" '^[A-Z][A-Z0-9_]*$'
validate_token "metric_prefix" "${metric_prefix}" '^[a-z][a-z0-9_]*$'
validate_token "image_prefix" "${image_prefix}" '^[a-z][a-z0-9-]*$'
validate_token "display_name" "${display_name}" '^[A-Za-z][A-Za-z0-9]*$'

# 裸 slug 用于数据库名、namespace、域名标签和未加分隔符的测试数据。
# 保留 metric 前缀里的下划线，避免 cloud_motion 被收成没有分隔符的名字。
current_slug="${current_metric}"
new_slug="${metric_prefix}"
validate_token "current_slug" "${current_slug}" '^[a-z][a-z0-9_]*$'
validate_token "new_slug" "${new_slug}" '^[a-z][a-z0-9_]*$'
if [[ "${current_slug}" == "${new_slug}" && "${current_module}" == "${module_prefix}" && "${current_env}" == "${env_prefix}" && "${current_metric}" == "${metric_prefix}" && "${current_image}" == "${image_prefix}" && "${current_display}" == "${display_name}" ]]; then
  fail "新旧项目身份相同，没有可替换内容"
fi

if [[ "${dry_run}" -eq 1 && "${apply}" -eq 1 ]]; then
  fail "--dry-run 与 --apply 不能同时使用"
fi

if [[ "${skip_generate}" -eq 1 && "${apply}" -eq 0 ]]; then
  fail "--skip-generate 只在 --apply 时有意义"
fi

if [[ "${skip_go_check}" -eq 1 || "${skip_generate}" -eq 1 ]]; then
  if [[ "${apply}" -eq 1 && "${skip_generate}" -eq 0 ]]; then
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

# 长 token 先替换，避免裸 slug 把 github.com/cloud_motion 截成 github.com/cloudmotion。
replace_tree "${current_module}" "${module_prefix}"
replace_tree "${current_env}" "${env_prefix}"
replace_tree "${current_metric}_" "${metric_prefix}_"
replace_tree "${current_metric}." "${metric_prefix}."
replace_tree "${current_image}-" "${image_prefix}-"
# 只替换镜像仓库段，不替换 Go module。github.com/old/service 必须留给 module_prefix。
replace_tree "io/\${GITHUB_REPOSITORY_OWNER}/${current_image}/" "io/\${GITHUB_REPOSITORY_OWNER}/${image_prefix}/"
replace_tree "io/\${repository_owner}/${current_image}/" "io/\${repository_owner}/${image_prefix}/"
replace_tree ".com/${current_image}/" ".com/${image_prefix}/"
replace_tree "${current_display}" "${display_name}"
replace_tree "${current_slug}" "${new_slug}"
# 目录名使用镜像前缀（cloud-motion-user-service），不用去掉连字符的裸 slug。
rename_matching_paths "${current_image}" "${image_prefix}"

if [[ "${apply}" -eq 1 ]]; then
  write_identity
  if ! git -C "${repo_root}" diff --exit-code -- user-service/internal/shared/rbacbaseline/ids.go >/dev/null; then
    fail "系统 RBAC ID 被修改，已停止"
  fi
  if [[ "${skip_generate}" -eq 1 ]]; then
    printf 'rename-project: skipped go work sync and code generation\n'
  else
    regenerate
  fi
  trap - ERR
fi

printf 'rename-project: ok\n'
