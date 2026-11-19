#!/usr/bin/env bash
# 从模板创建新项目：更换内部标识，成功后删除 project.yaml、.github、.git、renovate.json 和 coverage/。
# 默认只预览；执行要求纯净工作区，失败时恢复 Git 跟踪内容。
# 保留业务服务目录和系统 RBAC ID；忽略文件不在 Git 恢复范围内。
#
# 执行示例（在仓库根目录运行）：
#   预览更名，不写文件：
#     ./scripts/rename-project.sh --dry-run --module github.com/example/my-project
#   应用更名，成功后清除模板配置和 Git 历史：
#     ./scripts/rename-project.sh --apply --module github.com/example/my-project
set -euo pipefail

# 项目路径
repo_root="$(git -C "$(dirname "${BASH_SOURCE[0]}")" rev-parse --show-toplevel)"
cd "${repo_root}"
identity_file="${repo_root}/project.yaml"

# 执行选项（默认只预览）
apply=0             # 是否实际写入文件
mode=""             # 记录显式模式，用于检查 --dry-run 与 --apply 冲突
skip_go_check=0     # 是否跳过本机 Go 版本检查
skip_generate=0     # 是否跳过 Ent 与 OpenAPI 代码重新生成

# 目标身份
name=""             # 新项目短名/标识（如 my-project），用于自动推导其他前缀
module=""           # 新项目 Go module path（如 github.com/org/repo）
module_prefix=""    # 显式指定的 module 前缀
env_prefix=""       # 显式指定的环境变量前缀（如 MY_PROJECT）
metric_prefix=""    # 显式指定的指标前缀（如 my_project）
image_prefix=""     # 显式指定的镜像前缀（如 my-project）
display_name=""     # 显式指定的展示名称（如 MyProject）

# 旧身份覆盖值，用于清理历史残留。
from_name=""
from_module=""
from_env=""
from_metric=""
from_image=""
from_display=""

# 累积规则后一次替换，避免新名称包含旧名称时被后续规则再次修改。
replacement_rules=""
replacement_patterns=()

# 重定向到文件或管道时不输出颜色。
if [[ -t 1 ]]; then
  c_reset=$'\033[0m'
  c_bold=$'\033[1m'
  c_dim=$'\033[2m'
  c_red=$'\033[31m'
  c_green=$'\033[32m'
  c_yellow=$'\033[33m'
  c_blue=$'\033[34m'
  c_cyan=$'\033[36m'
else
  c_reset=''
  c_bold=''
  c_dim=''
  c_red=''
  c_green=''
  c_yellow=''
  c_blue=''
  c_cyan=''
fi

# 打印信息日志
log_info() {
  printf '%sℹ [INFO]%s %s\n' "${c_blue}" "${c_reset}" "$*"
}

# 打印阶段进度大标题（如 [进度 1/6]）
log_step() {
  printf '\n%s%s▶ [进度 %s]%s %s\n' "${c_bold}" "${c_cyan}" "$1" "${c_reset}" "$2"
}

# 打印细分子任务进度
log_substep() {
  printf '  %s↳%s %s\n' "${c_dim}" "${c_reset}" "$*"
}

# 打印成功日志
log_success() {
  printf '%s✔ [成功]%s %s\n' "${c_green}" "${c_reset}" "$*"
}

# 打印警告日志
log_warn() {
  printf '%s⚠ [警告]%s %s\n' "${c_yellow}" "${c_reset}" "$*"
}

# 打印错误日志并输出至标准错误流
log_error() {
  printf '%s✖ [错误]%s %s\n' "${c_red}" "${c_reset}" "$*" >&2
}

# 遇到不可恢复错误时打印红色提示并以状态码 2 终止退出
fail() {
  log_error "$*"
  exit 2
}
# 命名风格转换辅助函数
# 使用 sed、awk 和 tr，兼容 macOS 自带的 Bash 3.2。

# 转为小写下划线命名，例如 MyProject / my-project -> my_project。
to_snake() {
  printf '%s' "$1" | sed -E 's/([a-z0-9])([A-Z])/\1_\2/g; s/[- ]+/_/g' | tr '[:upper:]' '[:lower:]'
}

# 转为小写连字符命名，例如 MyProject / my_project -> my-project。
to_kebab() {
  printf '%s' "$1" | sed -E 's/([a-z0-9])([A-Z])/\1_\2/g; s/[_ ]+/-/g' | tr '[:upper:]' '[:lower:]'
}

# 转为大写下划线命名，例如 my-project -> MY_PROJECT。
to_screaming_snake() {
  to_snake "$1" | tr '[:lower:]' '[:upper:]'
}

# 转为大驼峰命名，例如 my-project / my_app -> MyProject / MyApp。
to_pascal() {
  to_snake "$1" | awk -F '_' '{
    res = ""
    for (i = 1; i <= NF; i++) {
      if (length($i) > 0) {
        res = res toupper(substr($i, 1, 1)) tolower(substr($i, 2))
      }
    }
    print res
  }'
}

# 命令行帮助信息
usage() {
  cat <<'EOF'
项目全局更名工具 (rename-project.sh)

用法：
  推荐简便用法（自动推导环境变量、指标、镜像前缀和展示名）：
    ./scripts/rename-project.sh --dry-run --module github.com/example/my-project
    ./scripts/rename-project.sh --apply   --module github.com/example/my-project

  指定自定义项目短名（覆盖默认从 module 末尾自动提取的项目名称）：
    ./scripts/rename-project.sh --apply   --module github.com/example/my-project --name my-project

  完整显式用法（逐项显式覆盖五个内部标识）：
    ./scripts/rename-project.sh --apply \
      --module-prefix github.com/example/my-project \
      --env-prefix MY_PROJECT \
      --metric-prefix my_project \
      --image-prefix my-project \
      --display-name MyProject

选项参数说明：
  --dry-run            仅扫描并预览变更，不写入任何文件（默认模式）
  --apply              执行更名，成功后删除 project.yaml、.github、.git、renovate.json 和 coverage/（要求纯净的独立 Git 仓库）
  -m, --module <path>   新项目的 Go module path（支持 GitHub、GitLab、私有域名等多级路径）
  -n, --name <slug>     新项目名称标识（如 my-project），用于自动推导其他四个前缀
  --skip-go-check      跳过本机 Go 版本检查
  --skip-generate      跳过 go work sync、Ent 与 OpenAPI 重新生成（仅在缺少完整 Go 环境且需补清文本时使用）

清理历史残留（正文仍有旧标识时）：
  ./scripts/rename-project.sh --apply \
    --from-name old-app \
    --from-module-prefix github.com/old/old-app \
    --module github.com/new/new-app

三层名称边界说明：
  1. 项目内部标识（Module、Env、Metric、Image、Display Name）：由脚本全量自动更新并维护一致性。
  2. 本地项目目录名：不直接在脚本内重命名（防止终端当前工作目录与 IDE 索引失效），成功后给出手动移动提示。
  3. Git 仓库：成功后删除旧历史和 remote 配置；新仓库由开发者按需自行初始化。
EOF
}

# 业务逻辑与核心处理函数

# 从 project.yaml 读取当前身份，避免受外部环境变量影响。
read_identity() {
  local key="$1"
  local value
  value="$(awk -F': ' -v key="${key}" '$1 == key {print $2; exit}' "${identity_file}")"
  if [[ -z "${value}" ]]; then
    fail "配置文件 ${identity_file} 中缺少必要字段: ${key}"
  fi
  printf '%s' "${value}"
}

# 写入前检查命名，避免破坏 Go module、环境变量和镜像名称。
validate_token() {
  local name="$1"
  local value="$2"
  local pattern="$3"
  if [[ ! "${value}" =~ ${pattern} ]]; then
    fail "${name} 格式不合法: '${value}' (必须匹配规则: ${pattern})"
  fi
}

# 检查本机 Go 工具链是否就绪并输出版本
require_go() {
  local version_output
  if ! version_output="$(go version 2>&1)"; then
    fail "本机 Go 工具链无法正常执行！请确保已安装与 go.work 匹配的 Go 环境，且命令行能够调用 'go version'。原始错误: ${version_output}"
  fi
  log_substep "Go 工具链版本: ${version_output}"
}

# 收集字面量替换规则，保持较具体的规则优先。
add_replacement() {
  # 未变化的长名称也参与匹配，保护显式保留的 module 等字段不被短名称规则改写。
  replacement_patterns+=(-e "$1")
  replacement_rules+="$1"$'\t'"$2"$'\n'
}

# 只扫描一次文本，最长源串优先，新写入的身份不会再次参与匹配。
replace_contents() {
  local file matches status
  local files=()
  matches="$(mktemp)"
  if git grep -I -z -l -F "${replacement_patterns[@]}" -- . > "${matches}"; then
    :
  else
    status="$?"
    if [[ "${status}" -ne 1 ]]; then
      rm -f "${matches}"
      return "${status}"
    fi
  fi
  while IFS= read -r -d '' file; do
    case "${file}" in
      user-service/internal/persistence/ent/schema/*) ;;
      user-service/internal/persistence/ent/*|\
      user-service/docs/openapi.go|user-service/docs/openapi.json|user-service/docs/openapi.yaml)
        [[ "${skip_generate}" -eq 1 ]] || continue
        ;;
      user-service/internal/shared/rbacbaseline/ids.go) continue ;;
    esac
    # 不跟随仓库中的符号链接，避免改写仓库外的文件。
    [[ ! -L "${file}" ]] || continue
    files+=("${file}")
    log_substep "匹配文本: ${file}"
  done < "${matches}"
  rm -f "${matches}"
  log_substep "文本替换涉及 ${#files[@]} 个文件"
  if [[ "${apply}" -eq 1 && "${#files[@]}" -gt 0 ]]; then
    RULES="${replacement_rules}" perl -0pi -e '
      BEGIN {
        for (split /\n/, $ENV{RULES}) {
          my ($from, $to) = split /\t/, $_, 2;
          $to{$from} = $to unless exists $to{$from};
        }
        $pattern = join "|", map { quotemeta } sort { length($b) <=> length($a) } keys %to;
      }
      s/($pattern)/$to{$1}/g;
    ' -- "${files[@]}"
  fi
}

# 从跟踪文件推导待改名路径，由深至浅仅修改当前节点名称。
rename_matching_paths() {
  local from="$1" to="$2"
  [[ "${from}" != "${to}" ]] || return 0
  local path target base parent
  local paths=()
  while IFS= read -r -d '' path; do
    paths+=("${path}")
  done < <(git ls-files -z | FROM="${from}" perl -0ne '
    chomp;
    while (length) {
      my ($base) = m{([^/]+)$};
      $paths{$_} = 1 if index($base, $ENV{FROM}) >= 0;
      last unless s{/[^/]+$}{};
    }
    END { print "$_\0" for sort { length($b) <=> length($a) || $a cmp $b } keys %paths }
  ')
  # Bash 3.2 开启 nounset 时，直接展开空数组会报错。
  for path in "${paths[@]+"${paths[@]}"}"; do
    base="${path##*/}"
    parent="${path%/*}"
    [[ "${parent}" != "${path}" ]] || parent=.
    target="${parent}/${base//${from}/${to}}"
    if [[ -e "${target}" || -L "${target}" ]]; then
      fail "路径改名目标已存在: ${path} -> ${target}"
    fi
    log_substep "重命名资源路径: ${path} -> ${target}"
    if [[ "${apply}" -eq 1 ]]; then
      git mv -- "${path}" "${target}"
    fi
  done
}

# 仅在写入阶段失败时恢复跟踪内容；忽略文件不在 Git 的恢复范围内。
rollback() {
  local exit_status="$1"
  trap - EXIT INT TERM
  log_error "执行失败，正在恢复 Git 工作区..."
  if git reset --hard HEAD && git clean -fd; then
    log_info "已恢复 Git 跟踪内容并清理新增的非忽略文件。"
  else
    log_error "自动回滚失败，请检查 git status 并手动恢复工作区。"
  fi
  exit "${exit_status}"
}

# 先单独生成 Ent 核心包，避免全量生成时引用尚未迁移到新 module 的代码。
regenerate() {
  log_substep "同步 Go 工作区模块依赖 (go work sync)..."
  (
    cd "${repo_root}"
    go work sync
    log_substep "增量预编译 Ent 持久化模型 (go generate ./internal/persistence/ent)..."
    go -C user-service generate ./internal/persistence/ent
    log_substep "全量生成 Ent 代码 (make user-service-generate)..."
    make user-service-generate >/dev/null
    log_substep "重新生成 OpenAPI 规范与 Swagger UI (make user-service-openapi-generate)..."
    make user-service-openapi-generate >/dev/null
  )
}

# 命令行参数解析主循环
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    -m|--module|-n|--name|--*-prefix|--display-name|--from-name|--from-display-name)
      [[ "$#" -ge 2 && -n "$2" && "$2" != -* ]] || fail "参数 $1 缺少值"
      ;;
  esac
  case "$1" in
    --dry-run|--apply)
      [[ -z "${mode}" || "${mode}" == "$1" ]] || fail "--dry-run 与 --apply 互斥，不能同时指定"
      mode="$1"
      if [[ "$1" == --apply ]]; then
        apply=1
      fi
      ;;
    --skip-go-check) skip_go_check=1 ;;
    --skip-generate) skip_generate=1 ;;
    -m|--module) module="${2:-}"; shift ;;
    -n|--name) name="${2:-}"; shift ;;
    --from-name) from_name="${2:-}"; shift ;;
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
    *) fail "未知命令行参数: $1 (运行 --help 查看用法说明)" ;;
  esac
  shift
done

log_step "1/6" "正在检查运行环境与 Git 工作区状态..."

[[ -f "${identity_file}" ]] || fail "未找到项目身份配置文件: ${identity_file}"

if [[ "${apply}" -eq 1 ]]; then
  [[ -d "${repo_root}/.git" ]] || fail "linked worktree 不支持 --apply：成功更名需要删除独立仓库的 .git 目录"
fi
command -v perl >/dev/null || fail "需要安装 Perl 才能执行项目更名"

if [[ "${skip_generate}" -eq 1 && "${apply}" -eq 0 ]]; then
  fail "--skip-generate 仅在配合 --apply 执行时才有意义"
fi

# 回滚会覆盖跟踪文件，因此写入前必须拒绝未提交的修改。
if [[ "${apply}" -eq 1 ]]; then
  if [[ -n "$(git -C "${repo_root}" status --porcelain)" ]]; then
    fail "当前 Git 工作区存在未提交或未跟踪的修改！请先提交 (git commit) 或暂存 (git stash) 后再运行 --apply，防止执行异常回滚时丢失代码。"
  fi
  log_substep "Git 工作区纯净度检查: 通过 (纯净无未提交修改)"
fi

# 检查 Go 工具链环境
if [[ "${skip_go_check}" -eq 1 || "${skip_generate}" -eq 1 ]]; then
  if [[ "${apply}" -eq 1 && "${skip_generate}" -eq 0 ]]; then
    fail "--apply 模式下不能跳过 Go 工具链版本检查"
  fi
  log_substep "Go 工具链版本检查: 已跳过 (--skip-go-check)"
else
  require_go
fi

log_success "运行环境与工作区检查就绪"

log_step "2/6" "解析并校验新旧项目身份契约..."

# 读取当前权威身份
current_module="$(read_identity module_prefix)"
current_env="$(read_identity env_prefix)"
current_metric="$(read_identity metric_prefix)"
current_image="$(read_identity image_prefix)"
current_display="$(read_identity display_name)"

# 归一化 module 与 module_prefix
if [[ -n "${module}" && -z "${module_prefix}" ]]; then
  module_prefix="${module}"
fi

# 未指定 --name 时，从 module 末段推导缺失字段，保留显式覆盖值。
if [[ -z "${name}" && -n "${module_prefix}" ]]; then
  if [[ -z "${env_prefix}" || -z "${metric_prefix}" || -z "${image_prefix}" || -z "${display_name}" ]]; then
    name="$(basename "${module_prefix}")"
    log_substep "未显式指定 --name，自动从 module 末端提取项目短名: '${name}'"
  fi
fi

# 自动推导未显式指定的前缀
if [[ -n "${name}" ]]; then
  [[ -z "${env_prefix}" ]] && env_prefix="$(to_screaming_snake "${name}")"
  [[ -z "${metric_prefix}" ]] && metric_prefix="$(to_snake "${name}")"
  [[ -z "${image_prefix}" ]] && image_prefix="$(to_kebab "${name}")"
  [[ -z "${display_name}" ]] && display_name="$(to_pascal "${name}")"
fi

# 处理历史残留修复参数 (--from-name) 的自动推导
if [[ -n "${from_name}" ]]; then
  [[ -z "${from_env}" ]] && from_env="$(to_screaming_snake "${from_name}")"
  [[ -z "${from_metric}" ]] && from_metric="$(to_snake "${from_name}")"
  [[ -z "${from_image}" ]] && from_image="$(to_kebab "${from_name}")"
  [[ -z "${from_display}" ]] && from_display="$(to_pascal "${from_name}")"
fi

# 显式覆盖历史源身份（用于补清残留）
if [[ -n "${from_module}" || -n "${from_env}" || -n "${from_metric}" ||
      -n "${from_image}" || -n "${from_display}" ]]; then
  [[ -n "${from_module}" && -n "${from_env}" && -n "${from_metric}" &&
     -n "${from_image}" && -n "${from_display}" ]] || fail "--from-* 参数必须完整提供（或配合使用 --from-name 与 --from-module-prefix）"
  current_module="${from_module}"
  current_env="${from_env}"
  current_metric="${from_metric}"
  current_image="${from_image}"
  current_display="${from_display}"
fi

# 前置有效性断言
[[ -n "${module_prefix}" ]] || fail "缺少 module_prefix！请通过 --module <path> 或 --module-prefix <path> 指定新项目的 Go module 路径"
[[ -n "${env_prefix}" && -n "${metric_prefix}" &&
   -n "${image_prefix}" && -n "${display_name}" ]] || fail "缺少项目名称或前缀！请通过 --name <slug> 指定项目名称，或完整提供五个前缀参数"

# 新旧身份使用相同格式，避免非法值进入文本规则或路径匹配。
module_pattern='^[a-zA-Z0-9][-a-zA-Z0-9_.]*(/[a-zA-Z0-9][-a-zA-Z0-9_.]*)+$'
env_pattern='^[A-Z][A-Z0-9_]*$'
metric_pattern='^[a-z][a-z0-9_]*$'
image_pattern='^[a-z][a-z0-9-]*$'
display_pattern='^[A-Za-z][A-Za-z0-9]*$'

validate_token "module_prefix" "${module_prefix}" "${module_pattern}"
validate_token "env_prefix" "${env_prefix}" "${env_pattern}"
validate_token "metric_prefix" "${metric_prefix}" "${metric_pattern}"
validate_token "image_prefix" "${image_prefix}" "${image_pattern}"
validate_token "display_name" "${display_name}" "${display_pattern}"

validate_token "current_module" "${current_module}" "${module_pattern}"
validate_token "current_env" "${current_env}" "${env_pattern}"
validate_token "current_metric" "${current_metric}" "${metric_pattern}"
validate_token "current_image" "${current_image}" "${image_pattern}"
validate_token "current_display" "${current_display}" "${display_pattern}"

if [[ "${current_module}" == "${module_prefix}" &&
      "${current_env}" == "${env_prefix}" &&
      "${current_metric}" == "${metric_prefix}" &&
      "${current_image}" == "${image_prefix}" &&
      "${current_display}" == "${display_name}" ]]; then
  fail "新旧项目身份完全一致，无需执行任何替换操作"
fi

log_substep "源身份 -> 新身份契约对照:"
log_substep "  - Module Path : ${current_module} -> ${c_green}${module_prefix}${c_reset}"
log_substep "  - Env Prefix  : ${current_env} -> ${c_green}${env_prefix}${c_reset}"
log_substep "  - Metric Pre  : ${current_metric} -> ${c_green}${metric_prefix}${c_reset}"
log_substep "  - Image Pre   : ${current_image} -> ${c_green}${image_prefix}${c_reset}"
log_substep "  - Display Name: ${current_display} -> ${c_green}${display_name}${c_reset}"
log_success "身份契约解析与格式校验通过"

if [[ "${apply}" -eq 0 ]]; then
  log_step "3/6" "全库扫描并预演文本替换规则 (Dry-Run 模式，不修改磁盘文件)..."
else
  log_step "3/6" "执行全库文本树精准替换 (Apply 模式)..."
  # 从首次写入到生成结束，失败或中断都需要回滚。
  trap 'status=$?; if [[ "${status}" -ne 0 ]]; then rollback "${status}"; fi' EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
fi

# Go module：同时覆盖普通路径和架构检查中的两种正则转义写法。
add_replacement "${current_module}" "${module_prefix}"
current_module_esc="${current_module//./\\.}"
module_prefix_esc="${module_prefix//./\\.}"
if [[ "${current_module_esc}" != "${current_module}" ]]; then
  add_replacement "${current_module_esc}" "${module_prefix_esc}"
fi
current_module_double_esc="${current_module//./\\\\.}"
module_prefix_double_esc="${module_prefix//./\\\\.}"
if [[ "${current_module_double_esc}" != "${current_module}" ]]; then
  add_replacement "${current_module_double_esc}" "${module_prefix_double_esc}"
fi

# 环境变量和指标前缀。
add_replacement "${current_env}" "${env_prefix}"
add_replacement "${current_metric}_" "${metric_prefix}_"
add_replacement "${current_metric}." "${metric_prefix}."

# 镜像路径：较长规则优先于下方的裸项目名，避免误用指标前缀。
add_replacement "${current_image}-" "${image_prefix}-"
add_replacement "${current_image}/" "${image_prefix}/"
add_replacement "io/\${GITHUB_REPOSITORY_OWNER}/${current_image}/" "io/\${GITHUB_REPOSITORY_OWNER}/${image_prefix}/"
add_replacement "io/\${repository_owner}/${current_image}/" "io/\${repository_owner}/${image_prefix}/"
add_replacement ".com/${current_image}/" ".com/${image_prefix}/"

# 展示名和裸项目名（数据库名、测试数据等）。
add_replacement "${current_display}" "${display_name}"
add_replacement "${current_metric}" "${metric_prefix}"

replace_contents

log_success "文本内容扫描与替换规则处理完毕"

log_step "4/6" "同步重命名匹配镜像前缀的文件与目录结构..."
rename_matching_paths "${current_image}" "${image_prefix}"

log_success "目录与资源路径调整完毕"

if [[ "${apply}" -eq 1 ]]; then
  log_step "5/6" "重新生成 Ent 与 OpenAPI 契约代码..."

  # 系统 RBAC ID 不能随项目名变化。
  if ! git -C "${repo_root}" diff --exit-code -- user-service/internal/shared/rbacbaseline/ids.go >/dev/null; then
    fail "安全检查失败: 检测到系统 RBAC UUID 文件 (ids.go) 被非预期修改，已紧急中止！"
  fi
  log_substep "系统 RBAC 内置 UUID 完整性校验: 通过 (未被篡改)"

  if [[ "${skip_generate}" -eq 1 ]]; then
    log_warn "已显式跳过 Go workspace 同步与代码重新生成 (--skip-generate)"
  else
    regenerate
    log_success "Ent 持久化代码与 OpenAPI 契约重新生成完毕"
  fi

  # 删除 Git 元数据后无法回滚到旧 HEAD，因此只在所有更名步骤成功后清理。
  trap - EXIT INT TERM
  log_step "6/6" "删除模板身份、模板配置与 Git 历史..."
  rm -rf -- "${identity_file}" "${repo_root}/.github" "${repo_root}/renovate.json" "${repo_root}/coverage" "${repo_root}/.git"
  log_success "已删除 project.yaml、.github、.git、renovate.json 和 coverage/，未初始化新 Git 仓库"
fi

# 执行结果与后续操作
repo_dir_name="$(basename "${repo_root}")"

if [[ "${apply}" -eq 1 ]]; then
  cat <<EOF

================================================================================
${c_bold}${c_green}✔ rename-project: 项目内部标识已成功重命名！${c_reset}
================================================================================
[已更新的项目内部标识]
  - Module Prefix : ${c_green}${module_prefix}${c_reset}
  - Env Prefix    : ${c_green}${env_prefix}${c_reset}
  - Metric Prefix : ${c_green}${metric_prefix}${c_reset}
  - Image Prefix  : ${c_green}${image_prefix}${c_reset}
  - Display Name  : ${c_green}${display_name}${c_reset}
EOF

  cat <<EOF

[模板清理结果]
  已删除 project.yaml、.github、.git、renovate.json 和 coverage/，原 Git 历史与 remote 配置已移除。
  如需创建新 Git 仓库，请自行执行：
    ${c_cyan}git init -b main && git add . && git commit -m "chore: initial commit"${c_reset}
EOF

  cat <<EOF

[本地项目目录名建议]
  当前本地目录为: ${repo_dir_name}。
  脚本未直接在内部移动当前运行中的根目录（以保护终端 cwd 句柄与 IDE 文件监听器）。
  如需将本地文件夹重命名为与新项目对齐，请在退出当前终端/IDE 后执行：
    ${c_cyan}cd .. && mv "${repo_dir_name}" "${image_prefix}" && cd "${image_prefix}"${c_reset}

[工程质量与健康度校验]
  推荐从根目录执行完整构建与测试门禁：
    ${c_cyan}make verify${c_reset}
================================================================================
EOF
else
  cat <<EOF

================================================================================
${c_bold}${c_yellow}✔ rename-project: Dry-Run 预演扫描完成！${c_reset}
================================================================================
  以上预览了在实际执行时将要进行的文本替换与资源重命名。
  实际更名成功后将删除 project.yaml、.github、.git、renovate.json 和 coverage/，不会初始化新 Git 仓库。
  当前磁盘未发生任何修改。
  确认无误后追加 ${c_bold}--apply${c_reset} 参数执行实际变更，例如：
    ${c_cyan}./scripts/rename-project.sh --apply --module ${module_prefix}${c_reset}
================================================================================
EOF
fi
