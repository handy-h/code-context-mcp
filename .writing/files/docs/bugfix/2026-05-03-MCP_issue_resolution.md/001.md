# MCP 启动问题分析与解决方案 - 可迁移模板
作者：你
日期：2026-05-03

本文件遵循 Assess → Route → Execute & Verify 的工作流程，提供一个可直接在新项目中复用的问题分析与解决方案模板，帮助快速诊断和解决 MCP（Code Context MCP）在本地/新项目中的启动与索引问题。

## 1) Assess（评估）
- 背景与范围
  - MCP 用于本地向量化索引，通过 Ollama 提供的模型服务实现代码上下文检索。
  - 在新环境中启动 MCP 时经常遇到启动失败/连接中断的错误，例如 -32000: Connection closed，且后续排查定位到缺少必须的环境变量 ZILLIZ_URI、ZILLIZ_TOKEN。
- 已知信息
  - opencode.json 配置中 mcp.code-context.env 指定了 ZILLIZ_URI、ZILLIZ_TOKEN、COLLECTION_NAME、PROJECT_PATH 等参数。
  - code_context_mcp 的启动应在注入这些环境变量后执行；直接执行可执行文件不会自动读取 opencode.json。
- 观察与证据
  - 直接执行 ./code_context_mcp 时，常见反馈为缺少环境变量。
  - 通过显式设置 ZILLIZ_URI/ZILLIZ_TOKEN 并执行 -index，索引通常可以成功完成（示例：51 个文件、1102 条向量）。
  - Ollama 服务在本地端口 11434 上可用，模型清单可访问。
- 目标
  - 提供一个可复用、自动化的启动流程，确保 MCP 启动时环境变量正确注入，支持在新项目中快速复用。

## 2) Route（路线）
- 相关参与者与分工
  - 实现执行：@fixer（实现 wrapper 脚本 start-mcp.sh，负责从 opencode.json 读取 env 并启动 MCP 子命令）
  - 文档与知识迁移：@librarian/你（完善快速启动指南、迁移要点）
  - 架构与风险评估：@oracle（如有必要提供设计层面的评估）
- 方案对比（简要）
  - 方案 A：通过启动包装器自动读取 opencode.json 并注入环境变量再启动 MCP（推荐）
  - 方案 B：手动提取 env 变量后导出再启动（快速但易错且不可复用）
  - 方案 C：修改 MCP 启动流程，使其自带读取 env 的能力（风险较高，变更大）
- 并行性考虑
  - 环境准备与 Ollama 健康检查等可以并行执行；核心实现包装器属于依赖任务，需要完成设计/实现后再进行验证。除非明确独立无依赖，否则不应并行推进依赖步骤。

## 3) Execute & Verify（执行与验证）
- 主要实现步骤（推荐顺序）
  1) 设计并实现一个 start-mcp.sh 包装器：
     - 功能：读取 opencode.json，提取 mcp.code-context.env 中的变量，导出为环境变量；根据传入的子命令（如 -index）转发执行到 code_context_mcp。
     - 安全性：不在日志中打印完整 token，必要时仅打印域名或掩码形式。
  2) 集成到现有工作流：替换直接执行 code_context_mcp 的使用场景，改为执行 ./start-mcp.sh -index "$PROJECT_PATH"。
  3) 测试验证：
     - 验证 start-mcp.sh 能正确注入 ZILLIZ_URI/ZILLIZ_TOKEN/PROJECT_PATH 等变量。
     - 启动 MCP：./start-mcp.sh -index "/path/to/project"，观察输出包含索引完成的统计信息。
     - 验证 Ollama 服务仍然可用（curl http://127.0.0.1:11434/v1/models）。
- 验证准则（验收标准）
  - Packaging：start-mcp.sh 能在不同环境中工作，并正确注入变量。
  - 功能：MCP 成功完成索引，输出明确的完成信息。
  - 安全：敏感信息不在日志中暴露（token 掩码处理）。

## 4) Migration & Reuse（迁移与复用）
- 适用场景
  - 新项目切换时需快速搭建 MCP 向量索引并与 Ollama/向量服务集成。
- 复用要点
  - 将 opencode.json 的 env 配置映射为 wrapper 的环境注入机制。
  - 保留现有 MCP 参数的转发逻辑，确保跨项目兼容性。
- 简易迁移步骤
  - 将 opencode.json 放置在新项目根目录，确保结构一致。
  - 使用 start-mcp.sh 启动脚本进行 MCP 启动与索引。
  - 更新文档中的快速启动指南与参数映射说明。

## 5) 风险与缓解
- 风险 1：token 泄露
  - 缓解：仅在运行时注入变量，日志不打印 token；需要时对 token 进行掩码处理。
- 风险 2：env 结构变化
  - 缓解：包装器对 opencode.json 的字段映射做成可配置，方便变更。
- 风险 3：跨平台/环境差异
  - 缓解：提供通用的 shell 脚本实现，必要时提供 Windows 脚本版本。

## 6) 变更记录
- 版本 1.0
  - 初始模板与实现思路
  - 将 wrapper 实现作为后续 patch 提交的候选项
- 版本 1.1（已实现 ✅）
  - 在 code-context-mcp 仓库根目录实现 start-mcp.sh 包装器
  - 支持自动向上搜索 opencode.json（当前目录 → 父目录 → $HOME/.config/opencode/）
  - 支持 jq / python3 双引擎解析 JSON，无需额外依赖
  - 敏感信息（TOKEN/SECRET/KEY）日志掩码处理
  - 支持 CONFIG_PATH / MCP_BINARY 环境变量覆盖
  - 通过 exec 转发所有参数，信号正确传递给 MCP 子进程
  - Makefile 新增 start-mcp / index-mcp 目标
  - 已验证：版本查询、env 注入、token 掩码、路径自动发现均正常

## 7) 附件与示例
- opencode.json 条目（mcp.code-context.env 的字段）示例：
  "env": {
    "OLLAMA_URL": "http://localhost:11434",
    "ZILLIZ_URI": "<your_uri>",
    "ZILLIZ_TOKEN": "<your_token>",
    "COLLECTION_NAME": "lab_record_go",
    "AUTO_INDEX": "true",
    "PROJECT_PATH": "/home/gao/Builds/Lab-record-go"
  }

## 8) 快速启动模板（示例）
- 手动启动流程（仅用于快速诊断，强烈建议改用 wrapper）:
  ```bash
  # 手动导出环境变量
  export ZILLIZ_URI="<your_uri>"
  export ZILLIZ_TOKEN="<your_token>"
  export PROJECT_PATH="/home/gao/Builds/Lab-record-go"
  # 启动并索引（二进制名由 make build 生成: code-context-mcp）
  ./code-context-mcp -index "$PROJECT_PATH"
  ```
- 使用 wrapper 的等效命令:
  ```bash
  # 自动从 opencode.json 读取 env 并启动（推荐）
  ./start-mcp.sh -index "$PROJECT_PATH"
  ```

## 9) 集成步骤（新项目 / 现有项目迁移）

### 步骤 1：配置 opencode.json
确保项目根目录包含以下结构的 opencode.json：
```json
{
  "mcp": {
    "code-context": {
      "type": "local",
      "command": ["./start-mcp.sh"],
      "env": {
        "OLLAMA_URL": "http://localhost:11434",
        "ZILLIZ_URI": "<your_uri>",
        "ZILLIZ_TOKEN": "<your_token>",
        "COLLECTION_NAME": "<your_collection>",
        "AUTO_INDEX": "true",
        "PROJECT_PATH": "/path/to/your/project"
      }
    }
  }
}
```

### 步骤 2：放置脚本
将 start-mcp.sh 和 code-context-mcp 放置在同一目录（或确保在 PATH 中）：
```bash
# 推荐：在项目根目录创建 symlink，二进制名与 make build 输出一致
ln -s /path/to/code-context-mcp/start-mcp.sh       ./start-mcp.sh
ln -s /path/to/code-context-mcp/code-context-mcp   ./code-context-mcp
```

### 步骤 3：验证
```bash
# 验证 env 注入和版本信息
./start-mcp.sh -version

# 构建索引
./start-mcp.sh -index "$PROJECT_PATH"
```

### 步骤 4：启动 opencode
此时 opencode 启动 MCP 时会通过 start-mcp.sh 自动注入 env 变量。
