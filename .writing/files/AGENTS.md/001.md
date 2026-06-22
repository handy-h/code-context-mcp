# AGENTS.md

## 项目概述

Go 实现的 MCP 服务器，通过 stdio JSON-RPC 2.0 为 AI 编码助手提供代码语义搜索。**MCP 协议是自行实现的，没有使用第三方 MCP SDK。**

## 常用命令

```bash
make build          # 编译到 cmd/code-context-mcp/code-context-mcp
make deploy         # 构建并部署到 code-text/（含 start-mcp.sh）
make test           # go test -race -coverprofile=coverage.out ./...
make vet            # go vet
make lint           # golangci-lint（配置在 .golangci.yml）
make fmt            # go fmt
```

Windows 环境用 `.\build.ps1`（默认构建+部署），参数：`-Test`、`-Lint`、`-Vet`、`-Fmt`、`-Clean`。

## 关键架构约束

### MCP stdio 协议
- stdin 读取 JSON-RPC 请求（每行一个），stdout 写响应，**日志必须写 stderr**
- 三者不能混用，否则会破坏 MCP 通信

### 配置系统
- `config.Config` 是 25 字段的全局配置结构，从环境变量加载（`internal/config/config.go`）
- `.env` 文件通过 `godotenv` 自动加载，搜索顺序：当前目录 → 可执行文件所在目录
- **修改配置前必须阅读 `docs/config-dependency-map.md`**，确认字段归属的子系统

### EMBEDDING_DIM 严格匹配
- `EMBEDDING_DIM` 必须与实际嵌入模型输出维度一致（Ollama nomic-embed-text: 768, OpenAI ada-002: 1536）
- Zilliz 集合创建后维度不可更改，本地 JSONL 切换维度需删除旧文件

### 向量存储后端
- `VECTOR_STORE` 环境变量控制：`local`/`local-jsonl`/`jsonl` → 本地 JSONL，`zilliz` → Zilliz Cloud
- 本地 JSONL 默认路径：可执行文件同目录下 `code-text/{COLLECTION_NAME}.jsonl`

### 倒排索引仅在内存中
- `InvertedIndex` 随服务重启丢失，由 `IndexManager.CheckAndAutoIndex` 重建
- `symbol_search` / `impact_analysis` 在索引未就绪时返回提示而非报错

## 包结构

| 路径 | 职责 |
|------|------|
| `cmd/code-context-mcp/main.go` | 入口：`-index` 模式或 MCP server 模式 |
| `internal/config/` | 配置加载（仅一个文件） |
| `internal/server/` | JSON-RPC 2.0 over stdio，工具定义注册 |
| `internal/tools/` | 5 个 MCP 工具的处理逻辑 |
| `internal/indexer/` | 文件扫描 + 向量化 + 写入存储 |
| `internal/search/` | VectorStore 接口 + LocalJSONLStore/Zilliz 实现 + 倒排索引 |
| `internal/embedding/` | EmbeddingProvider 接口，Ollama/OpenAI/Gemini 三种实现 |
| `pkg/structure/` | 按语言语法边界切分代码块 |
| `pkg/file/` | 文件结构摘要生成 |

## 部署

- `make deploy` 把二进制和 `start-mcp.sh` 部署到 `code-text/`
- `start-mcp.sh` 从模板生成（`start-mcp.sh.template`），为 OpenCode 注入 env 配置
- `code-text/` 是目标部署目录，本地 JSONL 索引文件也写在这里

## CI

- `.github/workflows/ci.yml`：push/PR 触发，go vet + 测试 + lint + build
- Go 版本矩阵：1.25、1.26（与 go.mod 的 `go 1.25.8` 对齐）
- `.github/workflows/release.yml`：tag 推送触发，多平台构建 + GitHub Release
- `.github/workflows/docker.yml`：多架构 Docker 镜像构建推送到 ghcr.io
