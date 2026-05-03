# code-context MCP 服务 — 实现方案

## 1. 架构设计

### 1.1 系统架构

```
┌─────────────────────────────────────────────────────┐
│                    MCPServer                         │
│                  (JSON-RPC 2.0)                      │
├─────────────────────────────────────────────────────┤
│                  工具注册层                          │
│  code_search | file_context | index_project          │
│  symbol_search | impact_analysis                     │
├─────────────────────────────────────────────────────┤
│                  索引子系统                          │
│  IndexManager | IndexStateStore | StructureSplitter  │
│  InvertedIndex | FileSummary                         │
├─────────────────────────────────────────────────────┤
│                  外部依赖                            │
│  VectorDB (Zilliz Cloud) | Ollama | Git | 本地文件   │
└─────────────────────────────────────────────────────┘
```

### 1.2 组件说明

| 组件 | 文件 | 职责 |
|------|------|------|
| IndexManager | index_manager.go | 索引生命周期管理：状态检测、全量/增量构建、过期判断 |
| IndexStateStore | index_state.go | 索引状态持久化：读写索引状态文件、指纹计算 |
| StructureSplitter | structure_splitter.go | 按语言语法结构切分代码 |
| InvertedIndex | inverted_index.go | 内存倒排索引：符号→位置映射 |
| FileSummary | file_summary.go | 文件结构摘要提取 |
| VectorDB | vectordb.go | 向量数据库封装（扩展 metadata + DeleteByFile） |
| MCPServer | server.go | MCP 协议服务器 |
| Tools | tools.go | 工具定义与处理器注册 |

### 1.3 数据流

**启动时自动索引：**
```
MCP启动 → IndexManager.CheckAndAutoIndex()
  → IndexStateStore.Load() 读取索引状态
  → 状态文件不存在 → 全量构建 BuildIndex()
  → 状态文件存在 → 比对 fingerprint
    → 一致 → 标记有效，重建内存倒排索引
    → 不一致 → 标记过期
```

**搜索时过期检测：**
```
code_search → IndexManager.IsStale()?
  → 过期 → goroutine 后台 IncrementalUpdate()
  → 同时用现有索引响应搜索请求
```

**结构切分：**
```
文件内容 → detectLanguage(filePath) → 按语言选择切分策略
  → 正则匹配语法结构边界 → 生成 []CodeChunk（含元数据）
  → 超长块二次切分
```

## 2. 详细设计

### 2.1 IndexManager

```go
type IndexManager struct {
    cfg         Config
    stateStore  *IndexStateStore
    invIndex    *InvertedIndex
    mu          sync.RWMutex
    stale       bool
    updating    atomic.Bool
    projectPath string
}
```

**核心方法：**
- `CheckAndAutoIndex(ctx)` — 启动时检查索引状态并自动索引
- `TriggerUpdateIfStale(ctx)` — 搜索时检测过期，触发后台增量更新
- `IsStale() bool` — 返回索引是否过期
- `GetInvertedIndex() *InvertedIndex` — 获取倒排索引实例

**索引指纹计算：**
1. 优先尝试 `git rev-parse HEAD` 获取 commit hash
2. 若非 git 仓库，计算所有目标文件的 mtime 摘要（SHA256）
3. 将指纹与 IndexState 中存储的指纹比对

**增量更新策略：**
1. 扫描当前文件 mtime，与 IndexState 对比找出变更文件
2. 从 VectorDB 中删除变更文件的旧向量（按 metadata.file 过滤）
3. 对变更文件重新切分、向量化、插入
4. 更新倒排索引和 IndexState

### 2.2 StructureSplitter

**语言检测：** `.go`→go, `.vue`→vue, `.js`→js, `.ts`→ts, `.md`→md, `.py`→py

**Go 切分：** 正则匹配 `^func\s+`, `^type\s+\w+\s+struct`, `^type\s+\w+\s+interface`, `^var\s+`, `^const\s+`

**Vue 切分：** 匹配 `<template>`, `<script>`, `<style>` 块边界；script 内按 JS/TS 策略二次切分

**JS/TS 切分：** 匹配 `export\s+function`, `export\s+class`, `function\s+`, `class\s+`, `const\s+\w+\s*=`

**Markdown 切分：** 匹配 `^#{1,6}\s+` 标题边界

**Python 切分：** 匹配 `^def\s+`, `^class\s+`

**超长块二次切分：** 超过 maxChunkSize（默认 1500 rune）的块按窗口二次切分

### 2.3 InvertedIndex

```go
type InvertedIndex struct {
    mu        sync.RWMutex
    index     map[string][]SymbolOccurrence  // 符号名 → 出现位置列表
    fileIndex map[string][]string            // 文件路径 → 符号列表
}
```

**索引构建：** 对每个 CodeChunk 用正则 `[a-zA-Z_][a-zA-Z0-9_]*` 提取标识符，symbol 元数据匹配的标记为 definition，否则为 reference

**驼峰/下划线转换：** `DiagnosisNotes` ↔ `diagnosis_notes`，搜索时同时查询

### 2.4 FileSummary

```go
type FileSummary struct {
    File      string     `json:"file"`
    Lines     int        `json:"lines"`
    Language  string     `json:"language"`
    Imports   []string   `json:"imports"`
    Functions []FuncInfo `json:"functions"`
    Types     []TypeInfo `json:"types"`
}
```

各语言提取 imports、functions（含行号范围）、types（struct/interface/class）

## 3. API 设计

### 3.1 工具列表

| 工具 | 参数 | 说明 |
|------|------|------|
| code_search | query, top_k | 语义搜索（增加过期检测） |
| file_context | file_path, mode(full/summary) | 文件内容或结构摘要 |
| index_project | path | 手动索引构建 |
| symbol_search | query, search_type(def/ref/all), top_k | 精确符号搜索 |
| impact_analysis | symbol, action(delete/rename/modify), new_name | 影响范围分析 |

### 3.2 symbol_search 输出示例

```
找到 3 个匹配位置：

--- config.go ---
  L10: [definition] Config struct {
  L32: [reference] func LoadConfig() Config {

--- tools.go ---
  L45: [reference] cfg Config
```

### 3.3 impact_analysis 输出示例

```json
{
  "symbol": "Config",
  "action": "delete",
  "impacts": [
    {
      "file": "config.go",
      "line": 10,
      "type": "definition",
      "context": "Config struct {",
      "suggestion": "删除该定义"
    }
  ],
  "summary": {
    "total_files": 2,
    "total_references": 3,
    "categories": {"definition": 1, "reference": 2}
  }
}
```

## 4. 存储设计

| 数据 | 存储方式 | 生命周期 |
|------|---------|---------|
| 向量索引 | Zilliz Cloud (Milvus) | 持久化，服务重启后保留 |
| 索引状态 | 本地 JSON 文件 `.code-context-index-state.json` | 持久化 |
| 倒排索引 | 进程内存 map | 随进程生命周期，重启时重建 |

## 5. 并发与错误处理

- **并发安全：** IndexManager.stale 用 sync.RWMutex 保护；InvertedIndex 用 sync.RWMutex 保护；后台更新用 atomic.Bool 确保单 goroutine
- **错误恢复：** 后台更新失败保持 stale=true 下次重试；状态文件损坏触发全量重建；VectorDB 不可用时 symbol_search/impact_analysis 仍可用内存倒排索引
