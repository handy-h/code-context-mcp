# Config 依赖地图

> 本文档用于 AI 助手快速了解项目中 `config.Config` 的 31 个字段分别被哪些子系统使用。
> **每次要求 AI 修改项目功能前，请 AI 先阅读此文档。**

## 概览

```
config.Config（31 个字段）
├── 嵌入模型相关（10个字段） → 仅 embedding 包使用
├── 向量存储相关（5个字段）  → 仅 search 包使用
├── 索引相关（6个字段）      → indexer 包使用
├── 超时相关（2个字段）      → tools.go 使用
└── Stats 相关（8个字段）    → tokenstats 包使用
```

## 各子系统实际依赖的配置字段

### 1. `internal/embedding/` — 嵌入模型

| 配置字段 | 使用的函数 | 用途 |
|----------|-----------|------|
| `EmbeddingProvider` | `NewEmbeddingProvider` | 选择用哪个模型（ollama/openai/gemini）|
| `OllamaURL` | `NewOllamaProvider` | Ollama 服务地址 |
| `OllamaModel` | `NewOllamaProvider` | Ollama 模型名 |
| `OpenAIBaseURL` | `NewOpenAIProvider` | OpenAI 兼容 API 地址 |
| `OpenAIModel` | `NewOpenAIProvider` | OpenAI 模型名 |
| `OpenAIAPIKey` | `NewOpenAIProvider` | OpenAI API 密钥 |
| `GeminiBaseURL` | `NewGeminiProvider` | Gemini API 地址 |
| `GeminiModel` | `NewGeminiProvider` | Gemini 模型名 |
| `GeminiAPIKey` | `NewGeminiProvider` | Gemini API 密钥 |
| `EmbeddingDim` | `NewOllama/OpenAI/GeminiProvider` | 向量维度 |

> ⚠️ **安全规则**：只有 embedding 包才能读取嵌入相关的配置，其他包不需要也不应该接触这些字段。

### 2. `internal/search/` — 向量数据库

| 配置字段 | 使用的函数 | 用途 |
|----------|-----------|------|
| `VectorStore` | `NewVectorDB` | 选择后端（local-jsonl / zilliz）|
| `VectorStorePath` | `NewLocalJSONLStore` | 本地存储路径 |
| `ZillizURI` | `NewZillizVectorDB` | Zilliz 连接地址 |
| `ZillizToken` | `NewZillizVectorDB` | Zilliz 认证令牌 |
| `CollectionName` | `VectorDB` 所有方法 | 集合名称 |
| `EmbeddingDim` | `VectorDB.EnsureCollection` / `Insert` | 创建集合时指定向量维度 |

> ⚠️ **安全规则**：只有 search 包才能读取向量存储配置。`EmbeddingDim` 是两个包共享的字段（embedding 和 search 都需要）。

### 3. `internal/indexer/` — 索引管理

| 配置字段 | 使用的函数 | 用途 |
|----------|-----------|------|
| `ScanExtensions` | `ScanFiles`、增量/全量构建的所有路径 | 扫描哪些文件后缀 |
| `ChunkSize` | `SplitByStructure` | 降级切分时的块大小 |
| `MaxChunkSize` | `SplitByStructure` | 单个代码块的最大大小 |
| `AutoIndex` | `CheckAndAutoIndex` | 是否在启动时自动索引 |
| `ProjectPath` | `NewIndexManager` | 要索引的项目路径 |
| `IndexStatePath` | `NewIndexStateStore` | 状态文件存储路径 |

### 4. `internal/tools/tools.go` — 工具注册层

| 配置字段 | 使用的函数 | 用途 |
|----------|-----------|------|
| `SearchTimeout` | `handleCodeSearch` | 搜索超时控制 |
| `IndexTimeout` | `handleIndexProject` | 索引超时控制 |

> 💡 注意：tools.go 本身**不直接**使用大部分配置，它把 `cfg` 传给 indexer、search、embedding 的子函数。

### 5. `internal/server/server.go` — MCP 服务器

> 💡 server.go 存储 `cfg` 但没有使用任何配置字段。保留它只是为了兼容工具注册的接口签名。新增的 `tracker` 字段通过 `SetTracker` 注入，不经过 Config。

### 6. `internal/tokenstats/` — Token 节省统计

| 配置字段 | 使用的函数 | 用途 |
|----------|-----------|------|
| `TokenStatsEnabled` | `main.go runMCPMode` | 是否启用统计 |
| `TokenStatsPath` | `NewStore` | 统计文件持久化路径 |
| `TokenStatsCharsPerToken` | `NewTracker` | 字符/token 转换系数 |
| `TokenStatsCodeSearchBaseline` | `NewTracker` → `BaselineConfig` | code_search 基线 |
| `TokenStatsFileContextBaseline` | `NewTracker` → `BaselineConfig` | file_context 基线 |
| `TokenStatsSymbolSearchBaseline` | `NewTracker` → `BaselineConfig` | symbol_search 基线 |
| `TokenStatsImpactAnalysisBaseline` | `NewTracker` → `BaselineConfig` | impact_analysis 基线 |
| `TokenStatsRetentionDays` | `NewTracker` | 统计数据保留天数 |

## 修改时的重要规则

1. **如果你要改嵌入模型（换供应商、改模型）**：只需要改 `NewEmbeddingProvider` 和配置文件，search 和 indexer 的代码无需触碰。
2. **如果你要改向量存储（换后端）**：只需要改 `NewVectorDB` 工厂函数和配置。
3. **如果你要改索引策略**：需要同时改 indexer 和配置文件。
4. **如果你要新增配置项**：先在 `config.go` 添加字段，然后只在你需要的子系统中读取它。不要为了图方便把新配置也传遍所有函数。
