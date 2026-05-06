#!/usr/bin/env bash
# ============================================================================
# start-mcp.sh — MCP 启动包装器
# 功能：从 opencode.json 中读取 mcp.code-context.env 环境变量，
#       注入到 code-context-mcp 进程后执行。
#
# 用法：
#   ./start-mcp.sh                     # 启动 MCP 服务器模式
#   ./start-mcp.sh -index <项目路径>   # 索引模式
#   ./start-mcp.sh -version            # 查看版本
#
# 环境变量：
#   MCP_BINARY   指定 code-context-mcp 路径（默认：本脚本所在目录下的 code-context-mcp）
#   CONFIG_PATH  指定 opencode.json 路径（默认：自动向上搜索）
# ============================================================================
set -euo pipefail

# ---- 颜色 / 样式 -----------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info()  { echo -e "${GREEN}[INFO]${NC}  $*" >&2; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*" >&2; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# ---- 路径解析 --------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MCP_BINARY="${MCP_BINARY:-${SCRIPT_DIR}/../cmd/code-context-mcp/code-context-mcp}"

# ---- opencode.json 查找 ----------------------------------------------------
# 依次搜索：CONFIG_PATH 环境变量 → 当前目录 → 逐级父目录 → $HOME/.config/opencode/
find_opencode_json() {
    local dir
    dir="$(pwd)"
    while [[ "$dir" != "/" ]]; do
        if [[ -f "$dir/opencode.json" ]]; then
            echo "$dir/opencode.json"
            return 0
        fi
        dir="$(dirname "$dir")"
    done
    # 最后尝试用户级配置
    if [[ -f "$HOME/.config/opencode/opencode.json" ]]; then
        echo "$HOME/.config/opencode/opencode.json"
        return 0
    fi
    return 1
}

if [[ -n "${CONFIG_PATH:-}" ]]; then
    CONFIG_FILE="$CONFIG_PATH"
elif [[ -f "opencode.json" ]]; then
    CONFIG_FILE="$(pwd)/opencode.json"
else
    CONFIG_FILE="$(find_opencode_json)" || {
        error "未找到 opencode.json"
        error "请将 start-mcp.sh 放置在包含 opencode.json 的项目目录中运行"
        error "或通过 CONFIG_PATH 环境变量指定路径"
        exit 1
    }
fi

if [[ ! -f "$CONFIG_FILE" ]]; then
    error "配置文件不存在: $CONFIG_FILE"
    exit 1
fi

info "使用配置文件: $CONFIG_FILE"

# ---- 提取环境变量 ----------------------------------------------------------
# 支持 jq（更快）或 python3（更通用）两种解析方式
extract_env_json() {
    if command -v jq &>/dev/null; then
        jq -r '
          .mcp["code-context"].env // .mcp.code_context.env // empty
          | to_entries[]
          | "\(.key)=\(.value)"
        ' "$CONFIG_FILE" 2>/dev/null && return 0
    fi

    if command -v python3 &>/dev/null; then
        python3 -c "
import json, sys
try:
    with open('$CONFIG_FILE') as f:
        data = json.load(f)
    env = data.get('mcp', {}).get('code-context', {}).get('env', {}) \
       or data.get('mcp', {}).get('code_context', {}).get('env', {})
    for k, v in env.items():
        print(f'{k}={v}')
except Exception as e:
    sys.exit(1)
" 2>/dev/null && return 0
    fi

    error "无法解析 JSON：需要安装 jq 或 python3"
    return 1
}

ENV_LINES="$(extract_env_json)" || exit 1

if [[ -z "$ENV_LINES" ]]; then
    warn "opencode.json 中未找到 mcp.code-context.env 配置，直接启动 MCP"
fi

# ---- 注入环境变量（敏感信息掩码后打印）------------------------------------
while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    key="${line%%=*}"
    value="${line#*=}"

    export "$key=$value"

    # 对敏感字段进行掩码处理
    if [[ "$key" == *TOKEN* || "$key" == *SECRET* || "$key" == *PASSWORD* || "$key" == *KEY* ]]; then
        if [[ ${#value} -ge 8 ]]; then
            masked="${value:0:4}...${value: -4}"
        else
            masked="****"
        fi
        info "导出 $key=$masked"
    else
        info "导出 $key=$value"
    fi
done <<< "$ENV_LINES"

# ---- 校验 MCP 二进制 -------------------------------------------------------
if [[ ! -x "$MCP_BINARY" ]]; then
    error "MCP 二进制文件不存在或不可执行: $MCP_BINARY"
    error "请先执行 make build 或在 cmd/code-context-mcp 目录下构建"
    exit 1
fi

info "启动 MCP: $MCP_BINARY $*"

# ---- 执行（exec 替换当前进程，信号直接传递给 MCP）--------------------------
exec "$MCP_BINARY" "$@"
