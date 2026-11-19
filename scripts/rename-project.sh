#!/usr/bin/env bash
# ==============================================================================
# 脚本名称: rename-project.sh
# 脚本作用: 专为框架克隆者设计，用于在克隆后快速将模板默认身份全局修改为新项目身份。
# 涉及范围:
#   1. Go module 路径前缀（go.mod、go.work、所有手写代码的 import 语句及架构校验正则）
#   2. 环境变量前缀（AEGISCORE_* -> NEW_NAME_*）
#   3. 监控指标命名空间与配置前缀（aegiscore_* -> new_name_*）
#   4. 容器镜像前缀与工作流路径（aegiscore/* -> new-name/*、Helm values 等）
#   5. 项目业务展示名称（AegisCore -> NewName）
#   6. 裸项目标识符（用于数据库名称、Kubernetes 命名空间、Grafana 默认面板路径等）
#   7. Helm Chart 与 Grafana Provisioning 目录/文件重命名
# 安全原则:
#   - 不移动业务服务目录（保持 user-service 与未来的平级服务目录结构不变）
#   - 不修改系统内置的 RBAC UUID（保持 user-service/internal/shared/rbacbaseline/ids.go 不动）
#   - 绝不篡改第三方依赖包名与版本
#   - 要求工作区纯净，异常时自动执行事务级完整回滚，防止工作区处于半死半活的脏状态
# ==============================================================================
set -euo pipefail

# ------------------------------------------------------------------------------
# 运行时全局变量与标志位定义
# ------------------------------------------------------------------------------
repo_root="$(git rev-parse --show-toplevel)"
identity_file="${repo_root}/project.yaml"

# 执行控制标志位
apply=0              # 是否实际写入文件（0: 仅预览，1: 执行修改）
dry_run=1            # 是否为预演模式（默认开启，保护开发者不会误触直接修改）
skip_go_check=0      # 是否跳过本机 Go 版本检查
skip_generate=0      # 是否跳过 Ent 与 OpenAPI 代码重新生成
drop_github=0        # 是否移除 .github/ 目录与 renovate.json
reset_git=0          # 是否在成功更名后删除旧 Git 历史并自动重新初始化仓库
drop_git=0           # 是否在成功更名后仅删除旧 .git 目录

# 用户输入的目标身份参数
name=""              # 新项目短名/标识（如 my-project），用于自动推导其他前缀
module=""            # 新项目 Go module path（如 github.com/org/repo）
module_prefix=""     # 显式指定的 module 前缀
env_prefix=""        # 显式指定的环境变量前缀（如 MY_PROJECT）
metric_prefix=""     # 显式指定的指标前缀（如 my_project）
image_prefix=""      # 显式指定的镜像前缀（如 my-project）
display_name=""      # 显式指定的展示名称（如 MyProject）

# 历史身份覆盖参数（仅用于修复残留时手动指定）
from_name=""
from_module=""
from_env=""
from_metric=""
from_image=""
from_display=""

# 跟踪本次被修改过的文件清单
changed_files=()

# ------------------------------------------------------------------------------
# 终端色彩支持与日志打印辅助函数
# ------------------------------------------------------------------------------
# 自动检测当前输出流是否为交互式 TTY 终端，非 TTY 环境下自动禁用 ANSI 颜色转义序列
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
  printf "${c_blue}ℹ [INFO]${c_reset} %s\n" "$*"
}

# 打印阶段进度大标题（如 [进度 1/6]）
log_step() {
  printf "\n${c_bold}${c_cyan}▶ [进度 %s]${c_reset} %s\n" "$1" "$2"
}

# 打印细分子任务进度
log_substep() {
  printf "  ${c_dim}↳${c_reset} %s\n" "$*"
}

# 打印成功日志
log_success() {
  printf "${c_green}✔ [成功]${c_reset} %s\n" "$*"
}

# 打印警告日志
log_warn() {
  printf "${c_yellow}⚠ [警告]${c_reset} %s\n" "$*"
}

# 打印错误日志并输出至标准错误流
log_error() {
  printf "${c_red}✖ [错误]${c_reset} %s\n" "$*" >&2
}

# 遇到不可恢复错误时打印红色提示并以状态码 2 终止退出
fail() {
  log_error "$*"
  exit 2
}

# ------------------------------------------------------------------------------
# 命名风格转换辅助函数
# 说明: 严格使用 sed、awk 和 tr 等 POSIX 标准工具，绝不依赖 Bash 4 的 ${var,,}，
#       确保在 macOS 默认自带的 Bash 3.2 及各类 Linux 发行版中均能稳定工作。
# ------------------------------------------------------------------------------

# 将任意风格转换为小写下划线命名（snake_case）
# 示例: MyProject / my-project -> my_project
to_snake() {
  printf '%s' "$1" | sed -E 's/([a-z0-9])([A-Z])/\1_\2/g; s/[- ]+/_/g' | tr '[:upper:]' '[:lower:]'
}

# 将任意风格转换为小写中划线连字符命名（kebab-case）
# 示例: MyProject / my_project -> my-project
to_kebab() {
  printf '%s' "$1" | sed -E 's/([a-z0-9])([A-Z])/\1_\2/g; s/[_ ]+/-/g' | tr '[:upper:]' '[:lower:]'
}

# 将任意风格转换为全大写下划线命名（SCREAMING_SNAKE_CASE）
# 示例: my-project -> MY_PROJECT
to_screaming_snake() {
  to_snake "$1" | tr '[:lower:]' '[:upper:]'
}

# 将任意风格转换为大驼峰帕斯卡命名（PascalCase）
# 示例: my-project / my_app -> MyProject / MyApp
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

# ------------------------------------------------------------------------------
# 命令行帮助信息
# ------------------------------------------------------------------------------
usage() {
  cat <<'EOF'
项目全局更名工具 (rename-project.sh)

用法：
  推荐简便用法（自动推导环境变量、指标、镜像前缀和展示名）：
    ./scripts/rename-project.sh --dry-run --module github.com/example/my-project
    ./scripts/rename-project.sh --apply   --module github.com/example/my-project

  克隆后一键更名并重置 Git 仓库（重命名验证通过后自动执行 rm -rf .git 并全新初始化）：
    ./scripts/rename-project.sh --apply   --module github.com/example/my-project --reset-git

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
  --dry-run             仅扫描并预览变更，不写入任何文件（默认模式）
  --apply               执行实际写入与更名操作（要求当前 Git 工作区纯净）
  -m, --module <path>   新项目的 Go module path（支持 GitHub、GitLab、私有域名等多级路径）
  -n, --name <slug>     新项目名称标识（如 my-project），用于自动推导其他四个前缀
  --drop-github         删除 .github/ 目录与 renovate.json（若新项目不使用 GitHub Actions / Renovate）
  --reset-git           重命名与代码生成验证完全通过后，自动重置 Git（rm -rf .git && git init -b main）
  --drop-git            重命名与代码生成验证完全通过后，仅直接删除 .git 目录（rm -rf .git）
  --skip-go-check       跳过本机 Go 版本检查
  --skip-generate       跳过 go work sync、Ent 与 OpenAPI 重新生成（仅在缺少完整 Go 环境且需补清文本时使用）

清理历史残留（project.yaml 已写入新身份，但代码仓库内仍有旧标识残留时）：
  ./scripts/rename-project.sh --apply \
    --from-name old-app \
    --from-module-prefix github.com/old/old-app \
    --module github.com/new/new-app

三层名称边界说明：
  1. 项目内部标识（Module、Env、Metric、Image、Display Name）：由脚本全量自动更新并维护一致性。
  2. 本地项目目录名：不直接在脚本内重命名（防止终端当前工作目录与 IDE 索引失效），成功后给出手动移动提示。
  3. 远程 Git 仓库：由开发者按需执行 git remote set-url origin，或使用 --reset-git 一键重新起步。
EOF
}

# ------------------------------------------------------------------------------
# 业务逻辑与核心处理函数
# ------------------------------------------------------------------------------

# 从权威身份源 project.yaml 中读取指定 key 的当前值
# 为什么这样做: 避免依赖不可靠的外部环境变量，确保每次替换都有明确单一的真实事实来源（Source of Truth）。
read_identity() {
  local key="$1"
  local value
  value="$(awk -F': ' -v key="${key}" '$1 == key {print $2; exit}' "${identity_file}")"
  if [[ -z "${value}" ]]; then
    fail "配置文件 ${identity_file} 中缺少必要字段: ${key}"
  fi
  printf '%s' "${value}"
}

# 正则格式有效性校验
# 为什么这样做: 在真正触碰文件前拦截非法命名，防止命名破坏 Go 编译、Linux 环境变量规范或 Docker 镜像标签规范。
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

# 记录被修改过的文件路径，用于审计与状态汇总
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

# 全局文本树扫描与精准替换引擎
# 关键设计与保护策略:
#   1. 使用 git grep -z -l -F 进行零字节分隔的绝对字面值匹配，防止空格或特殊符号被 shell 截断，且不走正则表达式转义。
#   2. 默认跳过 Ent 生成代码与 OpenAPI 生成产物，因为后续生成器会自动覆盖；但若启用了 --skip-generate，则必须替换，防止留存旧 module。
#   3. 坚决跳过 user-service/internal/shared/rbacbaseline/ids.go，保护已固化的系统超级管理员与内置角色 UUID 契约不变。
#   4. 使用 Perl 进行跨平台无损就地替换（Perl 在 macOS 和各类 Linux 上行为一致，完全避开 BSD sed 与 GNU sed 的 -i 兼容性地狱）。
replace_tree() {
  local from="$1"
  local to="$2"
  if [[ "${from}" == "${to}" ]]; then
    return
  fi
  local file
  local files=()
  while IFS= read -r -d '' file; do
    # 若用户指定 --drop-github，则跳过将被彻底丢弃的 GitHub Actions 与 Renovate 文件，避免无谓修改
    if [[ "${drop_github}" -eq 1 ]]; then
      case "${file}" in
        .github/*|.github|renovate.json) continue ;;
      esac
    fi

    case "${file}" in
      user-service/internal/persistence/ent/schema/*) ;; # Ent Schema 手写代码必须替换，否则新生成器加载时会找不到新 module
      user-service/internal/persistence/ent/*)
        # 默认模式下跳过生成代码，由后续 Ent generate 覆盖；但在 --skip-generate 时必须予以替换
        if [[ "${skip_generate}" -eq 0 ]]; then
          continue
        fi
        ;;
      user-service/internal/shared/rbacbaseline/ids.go) continue ;; # 严禁修改已固化的系统 RBAC UUID
      user-service/docs/openapi.go|user-service/docs/openapi.json|user-service/docs/openapi.yaml)
        # 默认跳过 OpenAPI 生成物；但在 --skip-generate 时必须予以替换
        if [[ "${skip_generate}" -eq 0 ]]; then
          continue
        fi
        ;;
    esac
    files+=("${file}")
  done < <(
    cd "${repo_root}"
    git grep -z -l -F -- "${from}" -- .
  )

  local count="${#files[@]}"
  if [[ "${count}" -eq 0 ]]; then
    return
  fi

  log_substep "替换 '${c_yellow}${from}${c_reset}' -> '${c_green}${to}${c_reset}' (涉及 ${c_bold}${count}${c_reset} 个文件)"
  if [[ "${apply}" -eq 1 ]]; then
    local changed
    for changed in "${files[@]}"; do
      record_changed "${changed}"
    done
    FROM="${from}" TO="${to}" perl -0pi -e 's/\Q$ENV{FROM}\E/$ENV{TO}/g' "${files[@]}"
  fi
}

# 目录与文件路径重命名引擎
# 关键设计与保护策略:
#   1. 不进入 .git 版本库目录。
#   2. 忽略由于上一步执行留下来的空目录，避免无效重命名。
#   3. 核心算法: 按路径长度逆序排序（由最深层节点向上层重命名），防止父目录被先更名后，子路径句柄瞬间失效报错。
#   4. 目标路径已存在时自动跳过，防止发生目录意外嵌套（例如将 source 嵌套移入 target/source）。
rename_matching_paths() {
  local from="$1"
  local to="$2"
  if [[ "${from}" == "${to}" ]]; then
    return
  fi
  local path relative target parent
  local paths=()

  # 收集包含目标名称的路径（排除 .git 目录）
  while IFS= read -r -d '' path; do
    if [[ -d "${path}" ]] && [[ -z "$(find "${path}" -mindepth 1 -print -quit)" ]]; then
      continue
    fi
    paths+=("${path}")
  done < <(find "${repo_root}" -name ".git" -prune -o -name "*${from}*" -print0)

  if [[ "${#paths[@]}" -eq 0 ]]; then
    return
  fi

  # 路径由深至浅排序处理
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

    # 目标路径已存在时不进行 git mv，以防发生意外嵌套
    if [[ -e "${repo_root}/${target}" ]]; then
      log_substep "跳过路径重命名: ${relative} -> ${target} (目标已存在)"
      continue
    fi

    log_substep "重命名资源路径: ${c_yellow}${relative}${c_reset} -> ${c_green}${target}${c_reset}"
    if [[ "${apply}" -eq 1 ]]; then
      parent="$(dirname "${target}")"
      if [[ "${parent}" != "." && ! -d "${repo_root}/${parent}" ]]; then
        mkdir -p "${repo_root}/${parent}"
      fi
      git -C "${repo_root}" mv "${relative}" "${target}"
    fi
  done
}

# 将新身份回写到项目根目录的权威配置文件 project.yaml 中
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
  log_substep "更新权威身份基准文件: project.yaml"
}

# 事务级异常回滚机制
# 为什么这样做: 
#   传统的 'git checkout -- files' 只能恢复文本文件，完全无法撤销 'git mv' 已经暂存的重命名或新增的未跟踪文件。
#   本脚本在执行前先确保工作区纯净，一旦中途出现任何异常，通过 Git 原生重置 100% 恢复到执行前状态，绝不留下破损垃圾。
rollback() {
  local exit_status="$1"
  printf '\n'
  log_error "执行过程中捕获异常！正在将 Git 工作区彻底重置回执行前状态..."
  git -C "${repo_root}" reset --hard HEAD >/dev/null 2>&1 || true
  git -C "${repo_root}" clean -fd >/dev/null 2>&1 || true
  log_info "工作区已安全还原，未发生数据损坏。"
  exit "${exit_status}"
}

# 重新生成 Ent 持久化代码与 OpenAPI 契约交付物
# 关键设计:
#   1. 先执行 go work sync，让多 module 工作区识别新 module 路径。
#   2. 先单独生成 user-service 的 Ent 核心包，避免全局生成器由于找不到新 Ent 包而发生编译崩溃。
#   3. 调用 Makefile 目标重新生成完整 Ent 模型与 OpenAPI 文档。
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

# ------------------------------------------------------------------------------
# 命令行参数解析主循环
# ------------------------------------------------------------------------------
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --dry-run) dry_run=1; apply=0 ;;
    --apply) dry_run=0; apply=1 ;;
    --skip-go-check) skip_go_check=1 ;;
    --skip-generate) skip_generate=1 ;;
    --drop-github) drop_github=1 ;;
    --reset-git) reset_git=1 ;;
    --drop-git) drop_git=1 ;;
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

# ------------------------------------------------------------------------------
# 阶段 [1/6] 检查运行环境与 Git 工作区纯净度
# ------------------------------------------------------------------------------
log_step "1/6" "正在检查运行环境与 Git 工作区状态..."

[[ -f "${identity_file}" ]] || fail "未找到项目身份配置文件: ${identity_file}"

if [[ "${dry_run}" -eq 1 && "${apply}" -eq 1 ]]; then
  fail "--dry-run 与 --apply 互斥，不能同时指定"
fi

if [[ "${skip_generate}" -eq 1 && "${apply}" -eq 0 ]]; then
  fail "--skip-generate 仅在配合 --apply 执行时才有意义"
fi

if [[ "${reset_git}" -eq 1 && "${drop_git}" -eq 1 ]]; then
  fail "--reset-git 与 --drop-git 不能同时指定，请二选一"
fi

# 在真正应用修改前，强制确保工作区纯净，防止中途异常回滚时覆盖开发者本地已有的未提交代码
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

# ------------------------------------------------------------------------------
# 阶段 [2/6] 解析并校验新旧项目身份契约
# ------------------------------------------------------------------------------
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

# 智能推导机制: 若未显式传入 --name，但提供了 module，且其余 4 个前缀未指定，则自动以 module 末尾段作为项目短名
if [[ -z "${name}" && -n "${module_prefix}" ]]; then
  if [[ -z "${env_prefix}" && -z "${metric_prefix}" && -z "${image_prefix}" && -z "${display_name}" ]]; then
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
if [[ -n "${from_module}" || -n "${from_env}" || -n "${from_metric}" || -n "${from_image}" || -n "${from_display}" ]]; then
  [[ -n "${from_module}" && -n "${from_env}" && -n "${from_metric}" && -n "${from_image}" && -n "${from_display}" ]] || fail "--from-* 参数必须完整提供（或配合使用 --from-name 与 --from-module-prefix）"
  current_module="${from_module}"
  current_env="${from_env}"
  current_metric="${from_metric}"
  current_image="${from_image}"
  current_display="${from_display}"
fi

# 前置有效性断言
[[ -n "${module_prefix}" ]] || fail "缺少 module_prefix！请通过 --module <path> 或 --module-prefix <path> 指定新项目的 Go module 路径"
[[ -n "${env_prefix}" && -n "${metric_prefix}" && -n "${image_prefix}" && -n "${display_name}" ]] || fail "缺少项目名称或前缀！请通过 --name <slug> 指定项目名称，或完整提供五个前缀参数"

# 格式校验: 放宽 module_prefix，支持各主流平台规范（如 github.com/org/repo、gitlab.com/group/sub/repo）
validate_token "module_prefix" "${module_prefix}" '^[a-zA-Z0-9][-a-zA-Z0-9_.]*(/[a-zA-Z0-9][-a-zA-Z0-9_.]*)+$'
validate_token "env_prefix" "${env_prefix}" '^[A-Z][A-Z0-9_]*$'
validate_token "metric_prefix" "${metric_prefix}" '^[a-z][a-z0-9_]*$'
validate_token "image_prefix" "${image_prefix}" '^[a-z][a-z0-9-]*$'
validate_token "display_name" "${display_name}" '^[A-Za-z][A-Za-z0-9]*$'

# 裸 slug 标识符校验（用于数据库名称、K8s 命名空间、无前缀测试数据等）
current_slug="${current_metric}"
new_slug="${metric_prefix}"
validate_token "current_slug" "${current_slug}" '^[a-z][a-z0-9_]*$'
validate_token "new_slug" "${new_slug}" '^[a-z][a-z0-9_]*$'

if [[ "${current_slug}" == "${new_slug}" && "${current_module}" == "${module_prefix}" && "${current_env}" == "${env_prefix}" && "${current_metric}" == "${metric_prefix}" && "${current_image}" == "${image_prefix}" && "${current_display}" == "${display_name}" ]]; then
  fail "新旧项目身份完全一致，无需执行任何替换操作"
fi

log_substep "源身份 -> 新身份契约对照:"
log_substep "  - Module Path : ${current_module} -> ${c_green}${module_prefix}${c_reset}"
log_substep "  - Env Prefix  : ${current_env} -> ${c_green}${env_prefix}${c_reset}"
log_substep "  - Metric Pre  : ${current_metric} -> ${c_green}${metric_prefix}${c_reset}"
log_substep "  - Image Pre   : ${current_image} -> ${c_green}${image_prefix}${c_reset}"
log_substep "  - Display Name: ${current_display} -> ${c_green}${display_name}${c_reset}"
log_success "身份契约解析与格式校验通过"

# ------------------------------------------------------------------------------
# 阶段 [3/6] 全库扫描并批量替换内部标识文本
# ------------------------------------------------------------------------------
if [[ "${dry_run}" -eq 1 ]]; then
  log_step "3/6" "全库扫描并预演文本替换规则 (Dry-Run 模式，不修改磁盘文件)..."
else
  log_step "3/6" "执行全库文本树精准替换 (Apply 模式)..."
  # 注册异常信号陷阱，一旦后续步骤抛错，触发事务级完整回滚
  trap 'rollback $?' ERR
fi

# 规则 1: 替换 Go Module 路径（同时处理单反斜杠与双反斜杠转义，覆盖常规代码与架构校验脚本中的正则字面量）
replace_tree "${current_module}" "${module_prefix}"
current_module_esc="${current_module//./\\.}"
module_prefix_esc="${module_prefix//./\\.}"
if [[ "${current_module_esc}" != "${current_module}" ]]; then
  replace_tree "${current_module_esc}" "${module_prefix_esc}"
fi
current_module_double_esc="${current_module//./\\\\.}"
module_prefix_double_esc="${module_prefix//./\\\\.}"
if [[ "${current_module_double_esc}" != "${current_module}" ]]; then
  replace_tree "${current_module_double_esc}" "${module_prefix_double_esc}"
fi

# 规则 2: 替换系统环境变量前缀（如 AEGISCORE_ -> MY_APP_）
replace_tree "${current_env}" "${env_prefix}"

# 规则 3: 替换监控指标与配置前缀（如 aegiscore_ -> my_app_、aegiscore. -> my_app.）
replace_tree "${current_metric}_" "${metric_prefix}_"
replace_tree "${current_metric}." "${metric_prefix}."

# 规则 4: 替换容器镜像前缀与 Registry 路径（优先替换镜像斜杠与横杠，防止后续裸词替换将镜像名误篡改为带下划线的 metric 前缀）
replace_tree "${current_image}-" "${image_prefix}-"
replace_tree "${current_image}/" "${image_prefix}/"
replace_tree "io/\${GITHUB_REPOSITORY_OWNER}/${current_image}/" "io/\${GITHUB_REPOSITORY_OWNER}/${image_prefix}/"
replace_tree "io/\${repository_owner}/${current_image}/" "io/\${repository_owner}/${image_prefix}/"
replace_tree ".com/${current_image}/" ".com/${image_prefix}/"

# 规则 5: 替换项目业务展示名（如 AegisCore -> MyApp）
replace_tree "${current_display}" "${display_name}"

# 规则 6: 替换裸项目标识 slug（用于覆盖 Makefile、测试数据、数据库库名与域名标签等）
replace_tree "${current_slug}" "${new_slug}"

log_success "文本内容扫描与替换规则处理完毕"

# ------------------------------------------------------------------------------
# 阶段 [4/6] 移动匹配镜像前缀的文件与目录
# ------------------------------------------------------------------------------
log_step "4/6" "同步重命名匹配镜像前缀的文件与目录结构..."
rename_matching_paths "${current_image}" "${image_prefix}"

# 可选清理: 根据用户意愿决定是否移除 GitHub Actions 与 Renovate 依赖配置
# 必须带 -f/--force 强制删除，防止此前文件存在就地修改时 git rm 报错中断退出
if [[ "${drop_github}" -eq 1 ]]; then
  if [[ -d "${repo_root}/.github" ]]; then
    log_substep "按用户参数要求移除 GitHub 持续集成配置目录: .github/"
    if [[ "${apply}" -eq 1 ]]; then
      git -C "${repo_root}" rm -rf --quiet -- .github 2>/dev/null || true
      rm -rf "${repo_root}/.github"
    fi
  fi
  if [[ -f "${repo_root}/renovate.json" ]]; then
    log_substep "按用户参数要求移除依赖自动化更新配置: renovate.json"
    if [[ "${apply}" -eq 1 ]]; then
      git -C "${repo_root}" rm -f --quiet -- renovate.json 2>/dev/null || true
      rm -f "${repo_root}/renovate.json"
    fi
  fi
fi
log_success "目录与资源路径调整完毕"

# ------------------------------------------------------------------------------
# 阶段 [5/6] 固化身份配置、重新生成代码与完整性校验
# ------------------------------------------------------------------------------
if [[ "${apply}" -eq 1 ]]; then
  log_step "5/6" "固化身份配置并重新生成 Ent 与 OpenAPI 契约代码..."
  
  # 步骤 5.1: 回写权威身份配置文件 project.yaml
  write_identity

  # 步骤 5.2: 完整性门禁断言，保护系统 RBAC UUID 严禁被篡改
  if ! git -C "${repo_root}" diff --exit-code -- user-service/internal/shared/rbacbaseline/ids.go >/dev/null; then
    fail "安全检查失败: 检测到系统 RBAC UUID 文件 (ids.go) 被非预期修改，已紧急中止！"
  fi
  log_substep "系统 RBAC 内置 UUID 完整性校验: 通过 (未被篡改)"

  # 步骤 5.3: 执行多模块同步与代码重新生成
  if [[ "${skip_generate}" -eq 1 ]]; then
    log_warn "已显式跳过 Go workspace 同步与代码重新生成 (--skip-generate)"
  else
    regenerate
    log_success "Ent 持久化代码与 OpenAPI 契约重新生成完毕"
  fi

  # 代码生成完毕且校验通过，解除错误信号陷阱，防止后续操作触发误回滚
  trap - ERR

  # ----------------------------------------------------------------------------
  # 阶段 [6/6] Git 仓库处置与最终交付确认
  # ----------------------------------------------------------------------------
  log_step "6/6" "Git 仓库历史处置与最终交付确认..."

  if [[ "${reset_git}" -eq 1 ]]; then
    log_substep "检测到 --reset-git，正在彻底剥离旧模板历史并重新初始化全新仓库..."
    rm -rf "${repo_root}/.git"
    git -C "${repo_root}" init -b main --quiet
    git -C "${repo_root}" add .
    git -C "${repo_root}" commit -m "chore: initialize project from template" --quiet
    log_success "已初始化全新 Git 仓库并创建主分支首个 commit"
  elif [[ "${drop_git}" -eq 1 ]]; then
    log_substep "检测到 --drop-git，正在删除旧模板 .git 目录..."
    rm -rf "${repo_root}/.git"
    log_success "已删除 .git 目录"
  else
    log_substep "保留原有 Git 历史与 remote 配置 (默认策略)"
  fi
fi

# ------------------------------------------------------------------------------
# 执行完成总结与后续操作操作建议报告
# ------------------------------------------------------------------------------
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
  - 身份配置文件  : project.yaml
EOF

  if [[ "${reset_git}" -eq 1 ]]; then
    cat <<EOF

[Git 仓库状态]
  已自动执行 rm -rf .git 并初始化全新 main 分支。关联您的远程新仓库只需运行：
    ${c_cyan}git remote add origin <your-new-remote-url>${c_reset}
    ${c_cyan}git push -u origin main${c_reset}
EOF
  elif [[ "${drop_git}" -eq 1 ]]; then
    cat <<EOF

[Git 仓库状态]
  已移除旧模板 .git 目录。如需初始化新仓库运行：
    ${c_cyan}git init -b main && git add . && git commit -m "chore: initial commit"${c_reset}
    ${c_cyan}git remote add origin <your-new-remote-url>${c_reset}
    ${c_cyan}git push -u origin main${c_reset}
EOF
  else
    cat <<EOF

[建议的 Git 仓库后续操作]
  若需切断旧模板历史全新起步，直接执行：
    ${c_cyan}rm -rf .git && git init -b main && git add . && git commit -m "chore: initial commit"${c_reset}
  若保留提交历史仅变更远端推送地址：
    ${c_cyan}git remote set-url origin <your-new-remote-url>${c_reset}
    ${c_cyan}git add . && git commit -m "chore: rename project from template"${c_reset}
    ${c_cyan}git push -u origin main${c_reset}
EOF
  fi

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
  当前磁盘未发生任何修改。
  确认无误后追加 ${c_bold}--apply${c_reset} 参数执行实际变更，例如：
    ${c_cyan}./scripts/rename-project.sh --apply --module ${module_prefix} --reset-git${c_reset}
================================================================================
EOF
fi
