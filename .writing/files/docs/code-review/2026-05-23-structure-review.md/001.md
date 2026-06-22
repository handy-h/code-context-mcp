# 代码结构审查报告（2026-05-23）

> **审查目标**：优化代码结构，减少 AI 工具在查询代码逻辑时所需的上下文读取量，提升 AI 的理解效率。
>
> **审查方法**：逐文件通读全部源代码，评估模块划分、函数抽象、代码扁平化、注释文档四个维度。

---

## 一、模块划分与职责边界

### 1. `server.go` 混合了 JSON-RPC 协议处理与 MCP 工具定义

**文件**：`internal/server/server.go`

当前 `server.go`（共 333 行）同时包含：
- **JSON-RPC 2.0 协议处理**（数据结构 + 通信循环）
- **MCP 工具定义**（`GetToolDefinitions()`，约 130 行）

AI 在理解"如何新增一个工具"时需要读完整个文件，其中混入了大量协议层细节（scanner 缓冲区大小、JSON 序列化等）。

**建议**：将 `GetToolDefinitions()` 和 `ToolDefinition` 结构体拆分到 `internal/server/tool_defs.go`，`server.go` 仅保留 MCP 协议逻辑。

**参考位置**：

```go
// internal/server/server.go:54-159 — 工具定义
// internal/server/server.go:205-332 — 协议通信循环
```

---

### 2. `RegisterTools` 函数过长，业务逻辑内联

**文件**：`internal/tools/tools.go`

`RegisterTools` 方法长达 **295 行**（第 44-339 行），每个工具的处理逻辑都以内联闭包形式写在同一个函数中。AI 要理解"code_search"的工作流必须找到对应的一段代码块（约 53 行）。

**建议**：每个工具拆分为独立函数（如 `handleCodeSearch`、`handleFileContext` 等），`RegisterTools` 仅做映射注册。

**参考位置**：

```go
// internal/tools/tools.go:46-99 — code_search handler 内嵌在 RegisterTools 中
```

---

### 3. `VectorStore` 接口定义与 Zilliz 实现在同一文件

**文件**：`internal/search/vectordb.go`

`VectorStore` 接口与 Zilliz `VectorDB` 结构体实现在同一个文件中，而 `LocalJSONLStore` 实现却在另一个文件 `local_jsonl_store.go` 中。AI 要对比两个实现时，需要先定位接口定义，再交叉比对。

**建议**：提取接口定义到独立的 `internal/search/contracts.go` 或 `internal/search/vector_store.go`。

**参考位置**：

```go
// internal/search/vectordb.go:17-25 — VectorStore 接口定义
```

---

### 4. `splitter.go` 单文件承载全部语言切分逻辑

**文件**：`pkg/structure/splitter.go`

该文件共 **673 行**，包含 Go / Vue / JS-TS / Markdown / Python 五种语言的切分策略。AI 理解"Go 切分逻辑"需要跳过约 400 行无关代码。

**建议**：按语言拆分为独立文件：
- `splitter_go.go`
- `splitter_vue.go`
- `splitter_js_ts.go`
- `splitter_md.go`
- `splitter_py.go`

`splitter.go` 仅保留 `SplitByStructure` 分发逻辑和通用辅助函数。

---

## 二、函数抽象与命名

### 5. `GetEmbedding` 隐式依赖全局单例

**文件**：`internal/embedding/embedding.go`

函数签名要求传入 `cfg` 参数，但实际使用 `sync.Once` 保证 provider 只初始化一次。后续调用即使传入不同的 `cfg` 也会被静默忽略。AI 无法从函数签名理解这一隐式行为。

**建议**：
- 方案 A：去掉 `sync.Once`，每次创建新 provider（开销极低）。
- 方案 B（推荐）：将单例管理移到 `main.go`，在启动时显式创建 `EmbeddingProvider`，通过依赖注入传递。

**参考位置**：

```go
// internal/embedding/embedding.go:28-37 — sync.Once 包裹 provider 初始化
```

---

### 6. 核心函数依赖"上帝 Config"参数

几乎所有核心函数都接收整个 `config.Config`：

- `BuildIndex(ctx, projectPath, cfg, vdb, invIndex)`
- `NewVectorDB(ctx, cfg)`
- `NewEmbeddingProvider(cfg)`

AI 无法从函数签名获知 `cfg` 中哪些字段被实际使用，必须深入函数体阅读才知道依赖哪些配置项。

**建议**：每个子系统定义自己的配置参数 struct，只传递需要的字段。例如：

```go
type IndexerConfig struct {
    ScanExtensions []string
    MaxChunkSize   int
    ProjectPath    string
}
```

---

### 7. 切分函数与摘要函数命名无法直观区分

| 文件 | 函数 | 实际用途 |
|------|------|----------|
| `pkg/structure/splitter.go` | `splitGo()` | 按结构切分代码 |
| `pkg/file/summary.go` | `extractGoSummary()` | 提取文件摘要 |

`split` 和 `extractSummary` 两个动词/动宾短语无法直观体现"切分"与"摘要"的本质区别。

**建议**：统一命名前缀约定：
- 切分类函数：`chunkGoSource` / `chunkFileByStructure`
- 摘要类函数：`summarizeGo` / `summarizeFile`

---

## 三、代码结构扁平化与解耦

### 8. 索引状态保存代码三处重复

**涉及文件**：
- `internal/tools/tools.go`（第 163-178 行）
- `internal/indexer/index_manager.go`（第 106-118 行）
- `internal/indexer/index_manager.go`（第 279-291 行）

三处位置包含几乎相同的索引状态保存逻辑：

```go
state := &types.IndexState{...}
if saveErr := stateStore.Save(state); saveErr != nil {
    slog.Warn("保存索引状态失败", "err", saveErr)
}
```

**建议**：在 `IndexStateStore` 上添加便捷方法 `SaveFromStats(stats, projectPath, currentMtimes)`，统一保存逻辑。

---

### 9. `server.go` 包含重复的分段注释行

**文件**：`internal/server/server.go`（第 13-15 行）

```go
// ================= JSON-RPC 2.0 数据结构 =================
// ================= JSON-RPC 2.0 数据结构 =================
```

明显的复制粘贴错误，可能导致 AI 误解代码结构。

**建议**：删除重复注释行。

---

### 10. `CodeChunk` 类型别名增加认知负担

**文件**：`pkg/structure/splitter.go`（第 11-12 行）

```go
type CodeChunk = types.CodeChunk
```

类型别名意味着 `structure.CodeChunk` 和 `types.CodeChunk` 是同一个类型。AI 在查阅代码时需要额外判断使用哪个包下的引用，增加了认知负载。

**建议**：去掉别名，统一使用 `types.CodeChunk`；或在规范中明确命名约定。

---

## 四、注释与文档

### 11. `config.go` 中文注释全部乱码

**文件**：`internal/config/config.go`

整个文件的中文注释全部显示为乱码（UTF-8 编码被错误解释为 Latin-1 后再次编码的结果），例如：

```go
// EmbeddingProviderType 宓屽叆妯″瀷鎻愪緵鍟嗙被鍨?
// Config MCP 鏈嶅姟鍣ㄥ叏灞€閰嶇疆
```

AI 完全无法读取这些注释。

**建议**：使用正确的 UTF-8 编码重新保存 `config.go`。

---

### 12. 关键切分策略缺乏高层注释解释

**文件**：`pkg/structure/splitter.go`

`splitGo` 函数中，`isContinuation` 的用途虽有简单单行注释，但对 `var/const` 括号块内多行声明的完整切分策略缺乏系统说明。AI 必须从多个正则表达式和分支逻辑中反向推断策略。

**建议**：在 `splitGo` 函数前添加整体策略说明，描述代码块边界判断规则。

---

### 13. 设计文档与代码实现路径不一致

**文件**：
- 文档：`docs/DESIGN.md`
- 代码：`internal/types/types.go`

设计文档中描述的 `FileSummary | file_summary.go`（`docs/DESIGN.md` 第 33 行），实际代码中类型定义在 `internal/types/types.go`，函数实现在 `pkg/file/summary.go`。文档路径与代码路径不匹配，AI 在文档和代码之间切换时会产生混淆。

**建议**：同步更新设计文档中的文件路径描述，使其与代码实际位置一致。

---

### 14. 倒排索引构建算法的核心假设缺少文档

**文件**：`internal/search/inverted_index.go`

`BuildFromChunks` 函数依赖 `chunk.Metadata["symbol"]` 来判断符号是"定义"还是"引用"节点。这个关键的前提假设——"`Metadata["symbol"]` 标识该块的所属符号"——在代码中没有任何文档说明。

**建议**：在文件头部添加核心假设说明：

```go
// InvertedIndex 符号倒排索引
//
// 构建规则：
//   - 每个 CodeChunk 的 Metadata["symbol"] 标识该块的"所属符号"（函数名、类型名等）
//   - 块内出现的标识符若等于 chunkSymbol → 标记为 definition
//   - 块内出现的其他标识符 → 标记为 reference
```

---

## 五、优先级汇总

| 优先级 | 编号 | 问题 | 影响 |
|--------|------|------|------|
| **P0** | #11 | `config.go` 中文注释全部乱码 | 所有中文注释 AI 不可读 |
| **P0** | #5 | `GetEmbedding` 隐式全局单例 | 函数契约不可信 |
| **P1** | #1 | `server.go` 混合协议与工具定义 | 跨文件追踪成本高 |
| **P1** | #2 | `RegisterTools` 函数 295 行 | 无法独立理解单个工具 |
| **P1** | #6 | "上帝 Config" 传递 | 函数签名不揭示真实依赖 |
| **P1** | #4 | `splitter.go` 673 行单文件 | 理解单一语言需要跳过大量无关代码 |
| **P2** | #3 | `VectorStore` 接口与实现同文件 | 实现对比需跨文件 |
| **P2** | #8 | 索引状态保存代码三处重复 | 修改需同步多处 |
| **P2** | #9 | `server.go` 重复注释行 | 误导代码结构理解 |
| **P2** | #14 | 倒排索引构建假设缺少文档 | 核心契约隐形 |
| **P3** | #7 | 切分/摘要函数命名不统一 | 区分两个概念开销大 |
| **P3** | #10 | `CodeChunk` 类型别名 | 认知负担 |
| **P3** | #13 | 设计文档与代码路径不一致 | 文档引导偏离 |

---

## 六、总结

该项目内核设计合理（配置→索引构建→搜索→MCP 协议三层架构），但以"AI 友好"为目标审视，存在三个核心问题：

1. **单文件过长**：`splitter.go`（673 行）、`tools.go`（339 行）让 AI 理解局部逻辑时必须读取大量无关上下文。
2. **隐式约定过多**：全局单例、上帝 Config、元数据字段约定都是"只有开发者知道"的隐性知识。
3. **职责耦合**：服务器协议、工具定义、业务逻辑混合在同一文件/函数中。

建议按 P0 → P3 优先级逐步修复。P0 级别的两个问题（注释乱码、全局单例）修复成本低、收益高，应优先处理。
