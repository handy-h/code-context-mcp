# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目简介

`code-context-mcp` 是一个用 Go 编写的 MCP (Model Context Protocol) 服务器，为 AI 编码助手提供代码语义搜索、符号查找、影响分析等能力。通过 stdio 上的 JSON-RPC 2.0 协议与 AI 工具通信，**MCP 协议是自行实现的，没有使用第三方 MCP SDK**。

## 常用命令

```bash
# 构建
make build           # 编译到 cmd/code-context-mcp/code-context-mcp
make deploy          # 构建并部署到 code-text/ 目录（含 start-mcp.sh）
make deploy DEPLOY_DIR=/path/to/target  # 部署到指定目录

# 测试与质量
make test            # go test -race -coverprofile=coverage.out ./...
make vet             # go vet ./...
make lint            # golangci-lint run ./...
make fmt             # go fmt ./...

# 命令行索引模式（构建完成后）
./cmd/code-context-mcp/code-context-mcp -index /path/to/project
./cmd/code-context-mcp/code-context-mcp -version

# 交叉编译
make build-all       # Linux/macOS/Windows 全平台
```

## 架构概览

### 两种运行模式

`main.go` 根据命令行参数选择模式：

- **`-index` 模式**：一次性扫描项目并建立索引，然后退出
- **MCP Server 模式**（默认）：启动 stdio JSON-RPC 服务，同时后台异步触发自动索引

### 数据流

```
AI 工具 → stdio JSON-RPC → server.go → tools.go
                                           ↓
                              ┌────────────┴────────────┐
                         向量搜索                    符号搜索
                   embedding.go → VectorDB         InvertedIndex
                   (Ollama/OpenAI/Gemini)     (内存，随 IndexManager 生命周期)
                              ↑                         ↑
                         indexer.go ← structure/splitter.go（按语法边界切分）
```

### 核心包

| 路径 | 职责 |
|------|------|
| `internal/server/server.go` | JSON-RPC 2.0 over stdio，工具定义注册 |
| `internal/tools/tools.go` | 5 个 MCP 工具的处理逻辑 |
| `internal/indexer/indexer.go` | 文件扫描 + 向量化 + 写入存储 |
| `internal/indexer/index_manager.go` | 自动增量索引管理（含过期检测与后台更新） |
| `internal/indexer/index_state.go` | 索引状态持久化（`.code-context-index-state.json`） |
| `internal/search/vectordb.go` | `VectorStore` 接口 + `NewVectorDB` 工厂（按 `VECTOR_STORE` 分发） |
| `internal/search/local_jsonl_store.go` | 本地 JSONL 向量存储实现 |
| `internal/search/inverted_index.go` | 内存倒排索引，支持驼峰/下划线转换 |
| `internal/embedding/provider.go` | `EmbeddingProvider` 接口，含 Ollama/OpenAI/Gemini 三种实现 |
| `pkg/structure/splitter.go` | 按语言语法边界切分代码块（Go/Vue/JS/TS/Python/Markdown） |
| `pkg/file/summary.go` | 生成文件结构摘要（函数列表、类型、导入） |

### 关键设计约束

- **`EMBEDDING_DIM` 必须与模型输出维度严格一致**，否则向量存储/搜索会失败。Zilliz Collection 创建后维度不可更改，本地 JSONL 切换维度需删除旧文件重建索引。
- **倒排索引（`InvertedIndex`）仅在内存中**，服务重启后由 `IndexManager.CheckAndAutoIndex` 重建；`symbol_search` / `impact_analysis` 在索引未就绪时会返回提示信息而非报错。
- **MCP 服务器通过 `io.Stdin` 读取行**，每行一个 JSON-RPC 请求；响应写到 `Stdout`，日志写到 `Stderr`，三者不能混用。
- `start-mcp.sh` 是专为 OpenCode 设计的包装器，用于将 `opencode.json` 中的 `env` 字段注入进程；其他工具（Cursor、Claude Desktop 等）可以直接调用二进制。

### 向量存储后端

`VECTOR_STORE` 环境变量控制后端选择：

- `local` / `local-jsonl` / `jsonl` → `LocalJSONLStore`（默认，索引文件路径由 `VECTOR_STORE_PATH` 指定）
- `zilliz` → `VectorDB`（需要 `ZILLIZ_URI` + `ZILLIZ_TOKEN`）

## 配置系统使用规则

> ⚠️ **核心约束**：`config.Config` 是一个包含 25 个字段的全局配置结构，在不同子系统中实际用到的字段不同。
> **在修改任何功能之前，请先阅读 `docs/config-dependency-map.md`**，确认你只修改了目标子系统依赖的配置字段。

### 规则清单

1. **嵌入模型相关字段**（EmbeddingProvider, Ollama*, OpenAI*, Gemini*, EmbeddingDim）→ 只能在 `internal/embedding/` 中使用
2. **向量存储相关字段**（VectorStore*, Zilliz*, CollectionName, EmbeddingDim）→ 只能在 `internal/search/` 中使用
3. **索引相关字段**（ScanExtensions, MaxChunkSize, AutoIndex, ProjectPath, IndexStatePath）→ 只能在 `internal/indexer/` 中使用
4. **超时字段**（SearchTimeout, IndexTimeout）→ 只能在 `internal/tools/tools.go` 中使用
5. **新增配置项时**，明确该配置属于哪个子系统，不要放在全局 Config 中，除非所有子系统都需要它
