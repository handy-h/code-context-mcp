# AGENTS.md

## 项目概览

这是一个 Go 语言编写的 **MCP (Model Context Protocol) 服务器**，为 AI 编码助手提供代码上下文能力。项目通过语义搜索、符号查找、影响分析等功能，替代传统 Grep/Glob/Read 的组合使用，减少上下文窗口消耗。

## 核心架构

```
main.go (cmd/code-context-mcp/)
  ├── 两种运行模式：
  │   ├── MCP 服务器模式（stdio JSON-RPC）— 默认
  │   └── 索引模式（-index 参数）— 手动构建索引
  │
  ├── internal/server/      — MCP 协议层（JSON-RPC 2.0 服务器）
  ├── internal/tools/       — 6 个 MCP 工具处理器
  ├── internal/config/      — 环境变量加载与验证
  ├── internal/indexer/     — 索引生命周期管理
  ├── internal/search/      — 向量搜索 + 倒排索引
  ├── internal/embedding/   — 嵌入模型提供者（Ollama/OpenAI/Gemini）
  ├── internal/tokenstats/  — Token 节省统计（默认禁用）
  ├── internal/types/       — 共享数据结构
  └── pkg/                  — 可复用的语言无关库
      ├── file/summary.go   — 文件结构摘要提取
      └── structure/splitter.go — 按语法边界切分代码
```

### 数据流

1. **启动时自动索引**：`IndexManager.CheckAndAutoIndex()` → 加载/检测索引状态 → 全量/增量构建
2. **搜索时过期检测**：`TriggerUpdateIfStale()` 在 `code_search` 中调用，后台触发增量更新
3. **结构切分**：`SplitByStructure()` 按语言正则匹配语法边界，超长块二次切分
4. **符号搜索**：内存倒排索引（重启后重建，与向量索引异步同步）

## 常用命令

### 开发环境（Linux/macOS）

```bash
make build       # 构建二进制到 cmd/code-context-mcp/code-context-mcp
make test        # go test -v -race -coverprofile=coverage.out ./...
make lint        # 自动安装并运行 golangci-lint v1.64.8
make vet         # go vet ./...
make fmt         # go fmt ./...
make clean       # 清理构建产物
make dev         # 构建并以当前目录为项目路径运行索引模式
```

### Windows 环境

```powershell
# 直接使用 go 命令构建
go build -o code-context-mcp.exe ./cmd/code-context-mcp

# 运行测试
go test -v -race ./...

# 运行 linter
golangci-lint run ./...
```

### 手动索引

```bash
# 构建索引后退出
./code-context-mcp/code-context-mcp -index /path/to/project

# 或通过 MCP 工具调用 index_project
```

## 关键环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `EMBEDDING_PROVIDER` | `ollama` | 嵌入模型提供商：`ollama` / `openai` / `gemini` |
| `EMBEDDING_DIM` | `768` | 向量维度（需与模型匹配） |
| `VECTOR_STORE` | `local` | 向量存储：`local`/`local-jsonl`/`jsonl`（本地 JSONL）或 `zilliz` |
| `VECTOR_STORE_PATH` | 可执行文件同目录 `{COLLECTION_NAME}.jsonl` | 本地 JSONL 文件路径 |
| `SCAN_EXTENSIONS` | `.go,.vue,.js,.ts,.py,.md,.rs` | 扫描的文件扩展名 |
| `CHUNK_SIZE` | `800` | 降级切分时的块大小（rune） |
| `MAX_CHUNK_SIZE` | `1500` | 结构切分后超长块的最大 rune 数 |
| `AUTO_INDEX` | `true` | 是否启用自动索引 |
| `PROJECT_PATH` | 空 | MCP 模式下自动索引的项目路径 |
| `TOKEN_STATS_ENABLED` | `false` | 是否启用 token 节省统计 |

## 代码组织与约定

### 包结构

- `internal/`：内部业务逻辑，外部不可导入
- `pkg/`：可复用的库代码，理论上可被外部项目引用
- `cmd/code-context-mcp/`：程序入口

### 命名约定

- Go 标准命名：导出首字母大写，未导出小写
- 中文注释：所有非 trivial 的注释使用中文
- 文件后缀：`_test.go`、`_go.go`（语言特定实现）
- 工具名：snake_case（如 `code_search`、`impact_analysis`），注册在 `tools.RegisterTools()`

### 接口设计

- `search.VectorStore`：向量存储统一接口，支持本地 JSONL 和 Zilliz
- `embedding.EmbeddingProvider`：嵌入模型提供者接口，支持 Ollama/OpenAI/Gemini
- 编译期接口检查：`var _ Interface = (*Impl)(nil)`

## 重要模式与机制

### 1. 索引生命周期

```
IndexStateStore 持久化索引状态（git commit hash 或文件 mtime SHA256 指纹）
IndexManager 管理索引生命周期：
  - stale 状态：sync.RWMutex 保护
  - 后台更新：atomic.Bool 确保同一时间只有一个更新 goroutine
  - 增量更新失败回退全量重建
```

### 2. 向量存储双后端

- **LocalJSONLStore**（默认）：
  - 所有向量加载到内存（`records` 字段）
  - 搜索时内存线性余弦相似度计算
  - 文件追加写入（Append-only JSONL）
  - 删除文件时过滤并重写整个 JSONL
- **VectorDB (Zilliz)**：
  - Milvus SDK v2 客户端
  - 集合自动创建（`embedding_idx` AUTOINDEX）
  - 按文件删除向量：Milvus 表达式过滤 `metadata["file"] == "..."`

### 3. 代码切分策略

- 正则匹配语法边界（非完整 AST）
- 每种语言独立实现（`splitter_*.go` 文件）
- 超长块按 `MAX_CHUNK_SIZE`（默认 1500 rune）二次切分
- 降级策略：不支持的扩展名使用固定字符窗口切分

### 4. 并发安全

- **MCP 工具调用**：异步处理（goroutine），避免阻塞后续请求
- **stdout 写入**：`writeMu sync.Mutex` 保护 JSON-RPC 响应输出
- **倒排索引**：`sync.RWMutex` 读写锁
- **token 统计**：`sync.Mutex` 保护，条件落盘（30 秒间隔）
- **自动索引**：后台 goroutine 异步执行，不阻塞 MCP 启动

### 5. 安全：路径遍历防护

`handleFileContext` 实现了多层路径安全检查：
1. `filepath.Clean()` 规范化路径
2. 拒绝包含 `..` 的路径
3. 解析符号链接后重新验证真实路径是否在项目根目录内
4. 基于已验证的绝对路径读取文件

## 测试

```bash
go test -v -race -coverprofile=coverage.out ./...
```

- 使用 Go 标准测试框架
- `-race` 检测竞态条件
- 覆盖率报告生成到 `coverage.out`，可用 `go tool cover -html=coverage.out` 查看
- 测试文件与源文件同目录，`*_test.go` 命名

## Lint 与格式

- **golangci-lint** v1.64.8（启用：errcheck, gosimple, govet, ineffassign, staticcheck, unused, gocritic, gofmt, goimports, misspell, unconvert, gocyclo, revive）
- **gocyclo** 复杂度阈值 25
- **revive** 的 `exported` 规则已禁用（允许未导出的测试访问）
- 代码格式化：`go fmt ./...`

## 常见陷阱与注意事项

1. **倒排索引重启后丢失**：`IndexManager.invIndex` 是内存结构，服务重启后需通过 `rebuildInvertedIndex()` 重建。如果向量索引存在但倒排索引未同步，`symbol_search` 会返回空结果。

2. **LocalJSONLStore 内存占用**：本地 JSONL 模式下，所有向量会加载到内存。大项目（数万文件）可能占用数百 MB 内存。

3. **嵌入模型维度必须匹配**：Ollama nomic-embed-text 为 768 维，OpenAI text-embedding-ada-002 为 1536 维。不匹配时 `ValidateEmbeddingDim()` 会输出警告。

4. **环境变量加载顺序**：godotenv 先加载当前目录 `.env`，再加载可执行文件所在目录 `.env`。后者会覆盖前者。

5. **stdout 专用于 MCP 通信**：所有日志必须输出到 stderr（`slog` 已配置），不能向 stdout 输出任何内容。

6. **Git 指纹超时**：`getGitCommitHash` 有 5 秒超时，非 git 仓库自动降级为 mtime 指纹。

7. **Zilliz 集合创建**：本地 JSONL 模式使用 `DropCollection` 删除旧文件，Zilliz 模式调用 `client.DropCollection`，首次运行时集合不存在会被忽略。

8. **JSON-RPC 消息处理**：
   - `notifications/initialized`：通知类消息，不返回响应
   - `initialize`/`tools/list`/`ping`：同步处理（轻量级协议方法）
   - `tools/call`：异步处理（goroutine），避免阻塞

## 关键文件索引

| 文件 | 职责 |
|------|------|
| `cmd/code-context-mcp/main.go` | 程序入口，模式切换，依赖初始化 |
| `internal/server/server.go` | MCP JSON-RPC 服务器，工具注册，请求分发 |
| `internal/tools/tools.go` | 6 个工具处理器的具体实现 |
| `internal/indexer/index_manager.go` | 索引生命周期管理，全量/增量更新 |
| `internal/indexer/index_state.go` | 索引状态持久化，git/mtime 指纹计算 |
| `internal/search/vectordb.go` | 向量存储后端（Zilliz） |
| `internal/search/local_jsonl_store.go` | 本地 JSONL 向量存储 |
| `internal/search/inverted_index.go` | 内存倒排索引，符号搜索 |
| `internal/embedding/provider.go` | 嵌入模型提供者（Ollama/OpenAI/Gemini） |
| `internal/config/config.go` | 环境变量加载与默认值 |
| `pkg/structure/splitter.go` | 按语言结构切分代码 |
| `pkg/file/summary.go` | 文件结构摘要提取 |
| `docs/DESIGN.md` | 详细架构设计文档 |
| `docs/SPEC.md` | 产品需求规格 |
