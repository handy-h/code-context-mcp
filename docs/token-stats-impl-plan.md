# Token 节省统计 + 文件收束 落地方案

## 一、改动总览

本次改动涉及四个维度：

1. 新增 `internal/tokenstats/` 包，实现 token 节省统计（按日聚合，JSON 持久化）
2. 新增 `token_stats` MCP 工具，供 AI 助手查询统计
3. 将所有运行时生成文件收束到可执行文件所在目录（不再创建 `.code-context/` 子目录）
4. 部署目录从 `code-text` 改名为 `code-context-mcp`

## 二、目标文件布局

改动前（文件散落在两个位置）：

```
{PROJECT_PATH}/
  └── .code-context-index-state.json       ← 散落在用户项目里

{exe_dir}/.code-context/                   ← 运行时自动创建的隐藏子目录
  └── code_context.jsonl
```

改动后（全部收束到 exe 目录，无子目录）：

```
{exe_dir}/
  ├── code-context-mcp                     ← 二进制本身
  ├── start-mcp.sh                         ← 启动脚本
  ├── code_context.jsonl                   ← 向量存储
  ├── index-state.json                     ← 索引状态
  └── token-stats.json                     ← token 统计（启用时）
```

部署目录改名：`code-text/` → `code-context-mcp/`。

## 三、Phase 1 — 文件路径收束

### 3.1 向量存储路径（config.go）

修改 `defaultLocalVectorStorePath()`，去掉 `.code-context` 子目录嵌套：

```go
// 改动前（config.go L161-170）
func defaultLocalVectorStorePath(collectionName string) string {
    dir := "."
    if exe, err := os.Executable(); err == nil {
        dir = filepath.Dir(exe)
    }
    if filepath.Base(dir) != ".code-context" {
        dir = filepath.Join(dir, ".code-context")
    }
    return filepath.Join(dir, collectionName+".jsonl")
}

// 改动后
func defaultLocalVectorStorePath(collectionName string) string {
    dir := "."
    if exe, err := os.Executable(); err == nil {
        dir = filepath.Dir(exe)
    }
    return filepath.Join(dir, collectionName+".jsonl")
}
```

### 3.2 索引状态路径（index_state.go）

修改 `NewIndexStateStore` 的默认路径，从 `{PROJECT_PATH}/` 改到 `{exe_dir}/`：

```go
// 改动前（index_state.go L24-28）
func NewIndexStateStore(projectPath string, statePath string) *IndexStateStore {
    if statePath == "" {
        statePath = filepath.Join(projectPath, ".code-context-index-state.json")
    }
    return &IndexStateStore{filePath: statePath}
}

// 改动后
func NewIndexStateStore(projectPath string, statePath string) *IndexStateStore {
    if statePath == "" {
        dir := "."
        if exe, err := os.Executable(); err == nil {
            dir = filepath.Dir(exe)
        }
        statePath = filepath.Join(dir, "index-state.json")
    }
    return &IndexStateStore{filePath: statePath}
}
```

注意：`projectPath` 参数保留（不删除），供未来多项目场景使用。现有调用方无需修改。

### 3.3 部署目录改名

`build.ps1` L29：

```powershell
# 改动前
$DeployDir = "code-text"
# 改动后
$DeployDir = "code-context-mcp"
```

### 3.4 文档与配置示例批量替换

以下文件中的 `code-text` → `code-context-mcp`、`.code-context-index-state.json` → `index-state.json`：

| 文件 | 替换内容 |
|------|---------|
| `.gitignore` L15, L18 | `code-text/` → `code-context-mcp/`；删除 `.code-context-index-state.json` 行 |
| `AGENTS.md` L11, L37, L59, L61 | `code-text/` → `code-context-mcp/`；更新 JSONL 路径描述 |
| `CLAUDE.md` L14 | `code-text/` → `code-context-mcp/` |
| `README.md` L59, L97, L102, L147, L161, L253 | `code-text` → `code-context-mcp`；更新索引状态默认路径描述 |
| `.env.example` L69-70, L118-119 | 更新路径示例 |
| `configs/examples/.env.ollama.example` L31, L33, L70-71 | 更新路径 |
| `configs/examples/.env.openai.example` L34, L36, L73-74 | 更新路径 |
| `configs/examples/.env.openai-compatible.example` L29-30 | 更新路径 |
| `configs/examples/.env.gemini.example` L37, L39, L76-77 | 更新路径 |
| `docs/promotion.md` L71 | `code-text/` → `code-context-mcp/` |
| `index_state_test.go` L45 | 更新断言的预期路径 |

## 四、Phase 2 — tokenstats 包

### 4.1 新增文件

```
internal/tokenstats/
  ├── types.go           # 数据结构
  ├── estimator.go       # EstimateTokens(s string, charsPerToken float64) int
  ├── baseline.go        # BaselineConfig + EstimateSavedTokens()
  ├── store.go           # JSON 文件持久化（原子 rename）
  ├── tracker.go         # Tracker：并发安全 + 内存聚合 + 即时落盘
  ├── estimator_test.go  # 纯 ASCII / 中文混合 / 空字符串 / 系数可配
  ├── baseline_test.go   # 各工具基线 / file_context full=0 / top_k 截断 / 不为负
  ├── tracker_test.go    # 并发 100 次 -race / 聚合正确 / 禁用空操作
  └── store_test.go      # Load 不存在返空 / Save-Load 一致性
```

### 4.2 types.go

```go
package tokenstats

import "time"

// ToolStats 单工具总量聚合
type ToolStats struct {
    CallCount        int64 `json:"call_count"`
    TotalInputTokens int64 `json:"total_input_tokens"`
    TotalOutputTokens int64 `json:"total_output_tokens"`
    TotalSavedTokens int64 `json:"total_saved_tokens"`
    TotalDurationMs  int64 `json:"total_duration_ms"`
}

// DailyStats 单日聚合统计
type DailyStats struct {
    Date         string `json:"date"`
    CallCount    int64  `json:"call_count"`
    InputTokens  int64  `json:"input_tokens"`
    OutputTokens int64  `json:"output_tokens"`
    SavedTokens  int64  `json:"saved_tokens"`
}

// StatsSnapshot 全量快照（持久化结构）
type StatsSnapshot struct {
    Version          string                `json:"version"`
    CreatedAt        time.Time             `json:"created_at"`
    UpdatedAt        time.Time             `json:"updated_at"`
    TotalCalls       int64                 `json:"total_calls"`
    TotalInputTokens int64                 `json:"total_input_tokens"`
    TotalOutputTokens int64               `json:"total_output_tokens"`
    TotalSavedTokens int64                `json:"total_saved_tokens"`
    ByTool           map[string]*ToolStats `json:"by_tool"`
    Daily            []DailyStats          `json:"daily"`
}

// ToolCallRecord 单次调用记录（不持久化，传给 Tracker.Record）
type ToolCallRecord struct {
    ToolName   string
    Args       map[string]interface{}
    OutputText string
    DurationMs int64
    Timestamp  time.Time
}
```

### 4.3 estimator.go

```go
package tokenstats

import "math"

// EstimateTokens 字符加权法估算 token 数
// asciiChars / charsPerToken + nonAsciiChars / 1.5，向上取整
func EstimateTokens(s string, charsPerToken float64) int {
    if s == "" {
        return 0
    }
    var ascii, nonASCII int
    for _, r := range s {
        if r < 128 {
            ascii++
        } else {
            nonASCII++
        }
    }
    tokens := float64(ascii)/charsPerToken + float64(nonASCII)/1.5
    return int(math.Ceil(tokens))
}
```

`charsPerToken` 用 `float64`（不是 int），避免整数除法精度丢失。默认值 4.0。

### 4.4 baseline.go

```go
package tokenstats

// BaselineConfig 各工具的基线参数
type BaselineConfig struct {
    CodeSearchFileTokens   int // 默认 2000
    FileContextBaseline    int // 默认 3000
    SymbolSearchBaseline   int // 默认 8000
    ImpactAnalysisBaseline int // 默认 12000
}

// EstimateSavedTokens 计算单次调用节省量
// saved = max(0, baseline - outputTokens)
func (c BaselineConfig) EstimateSavedTokens(toolName string, args map[string]interface{}, outputTokens int) int {
    baseline := c.calcBaseline(toolName, args)
    saved := baseline - outputTokens
    if saved < 0 {
        return 0
    }
    return saved
}

func (c BaselineConfig) calcBaseline(toolName string, args map[string]interface{}) int {
    switch toolName {
    case "code_search":
        topK := extractTopK(args) // 未传或非法值→5，合法值→min(val,10)
        return topK * c.CodeSearchFileTokens
    case "file_context":
        mode, _ := args["mode"].(string)
        if mode == "summary" || mode == "" {
            return c.FileContextBaseline
        }
        return 0 // mode=full 无节省
    case "symbol_search":
        return c.SymbolSearchBaseline
    case "impact_analysis":
        return c.ImpactAnalysisBaseline
    default:
        return 0
    }
}

func extractTopK(args map[string]interface{}) int {
    v, ok := args["top_k"]
    if !ok {
        return 5
    }
    // JSON number 可能是 float64
    f, ok := v.(float64)
    if !ok || f <= 0 {
        return 5
    }
    k := int(f)
    if k > 10 {
        return 10
    }
    return k
}
```

不参与统计的工具（`index_project`、`token_stats`）由 tracker 层过滤，baseline 层不感知。

### 4.5 store.go

```go
package tokenstats

// Store JSON 文件持久化，原子写入
type Store struct {
    filePath string
}

func NewStore(filePath string) *Store
func (s *Store) Load() (*StatsSnapshot, error)  // 文件不存在返回空快照
func (s *Store) Save(stats *StatsSnapshot) error // os.WriteFile(tmp) + os.Rename
```

空快照初始化逻辑：`Version: "1"`，`CreatedAt: time.Now()`，`ByTool: make(map[string]*ToolStats)`，`Daily: nil`。

### 4.6 tracker.go

```go
package tokenstats

// Tracker 并发安全的统计记录器
type Tracker struct {
    mu            sync.Mutex
    store         *Store
    baseline      BaselineConfig
    charsPerToken float64
    enabled       bool
    snapshot      *StatsSnapshot
    retentionDays int
}

func NewTracker(store *Store, baseline BaselineConfig, charsPerToken float64, enabled bool, retentionDays int) *Tracker
func (t *Tracker) Record(rec ToolCallRecord) error   // 更新内存 + 落盘
func (t *Tracker) GetStats() StatsSnapshot           // 返回副本

// skipTracking 过滤不参与统计的工具
var skipTracking = map[string]bool{
    "index_project": true,
    "token_stats":   true,
}
```

`Record` 核心逻辑：

1. `if !t.enabled { return nil }` — 禁用时直接返回
2. `if skipTracking[rec.ToolName] { return nil }` — 过滤元操作
3. 估算 input/output tokens
4. 计算 saved tokens
5. 更新总量聚合 + by_tool 聚合
6. 更新 daily 聚合（按 `rec.Timestamp.Format("2006-01-02")` bucket）
7. 裁剪超过 `retentionDays` 的 daily 条目
8. `store.Save(snapshot)` 落盘

并发模型：工具调用在 goroutine 中执行，`Record` 是唯一写入口，`sync.Mutex` 保护。

## 五、Phase 3 — 集成改造

### 5.1 Config 新增字段（config.go）

```go
// Config 结构新增（在现有 Timeout 区块后面加 Stats 区块）
// Stats
TokenStatsEnabled               bool
TokenStatsPath                  string
TokenStatsCharsPerToken         float64
TokenStatsCodeSearchBaseline    int
TokenStatsFileContextBaseline   int
TokenStatsSymbolSearchBaseline  int
TokenStatsImpactAnalysisBaseline int
TokenStatsRetentionDays         int
```

`LoadConfig` 中新增 env 解析：

| env 变量 | 字段 | 默认值 |
|---------|------|--------|
| `TOKEN_STATS_ENABLED` | `TokenStatsEnabled` | `false` |
| `TOKEN_STATS_PATH` | `TokenStatsPath` | `{exe_dir}/token-stats.json` |
| `TOKEN_STATS_CHARS_PER_TOKEN` | `TokenStatsCharsPerToken` | `4.0` |
| `TOKEN_STATS_CODE_SEARCH_BASELINE` | `TokenStatsCodeSearchBaseline` | `2000` |
| `TOKEN_STATS_FILE_CONTEXT_BASELINE` | `TokenStatsFileContextBaseline` | `3000` |
| `TOKEN_STATS_SYMBOL_SEARCH_BASELINE` | `TokenStatsSymbolSearchBaseline` | `8000` |
| `TOKEN_STATS_IMPACT_ANALYSIS_BASELINE` | `TokenStatsImpactAnalysisBaseline` | `12000` |
| `TOKEN_STATS_RETENTION_DAYS` | `TokenStatsRetentionDays` | `90` |

`TOKEN_STATS_PATH` 默认值计算：与 `defaultLocalVectorStorePath` 同逻辑，`{exe_dir}/token-stats.json`。

注意：Config 当前 23 字段 + 8 = 31 字段。`config-dependency-map.md` 中写的 25 是错误的，应同步修正。

### 5.2 埋点（server.go handleToolsCall）

```go
// 改动前（server.go L237）
resultText, err := handler(params.Arguments)

// 改动后
start := time.Now()
resultText, err := handler(params.Arguments)
duration := time.Since(start)

if s.tracker != nil && err == nil {
    if recordErr := s.tracker.Record(tokenstats.ToolCallRecord{
        ToolName:   params.Name,
        Args:       params.Arguments,
        OutputText: resultText,
        DurationMs: duration.Milliseconds(),
        Timestamp:  start,
    }); recordErr != nil {
        slog.Warn("记录 token 统计失败", "err", recordErr)
    }
}
```

`MCPServer` 结构新增 `tracker *tokenstats.Tracker` 字段。`NewMCPServer` 签名不变——tracker 通过 `RegisterTool` 模式注入（见 5.3）。

### 5.3 工具注册（tools.go + tool_defs.go）

**tools.go** — `RegisterTools` 签名新增 tracker 参数：

```go
// 改动前
func RegisterTools(srv *server.MCPServer, cfg config.Config, indexMgr *indexer.IndexManager)

// 改动后
func RegisterTools(srv *server.MCPServer, cfg config.Config, indexMgr *indexer.IndexManager, tracker *tokenstats.Tracker)
```

新增 handler：

```go
func handleTokenStats(tracker *tokenstats.Tracker) server.ToolHandler {
    return func(args map[string]interface{}) (string, error) {
        if tracker == nil {
            return "统计功能未启用", nil
        }
        stats := tracker.GetStats()
        return formatStats(stats), nil
    }
}
```

`RegisterTools` 中注册：

```go
srv.RegisterTool("token_stats", handleTokenStats(tracker))
```

tracker 同时通过 `srv.SetTracker(tracker)` 注入 MCPServer（供埋点使用）。

**tool_defs.go** — `GetToolDefinitions` 新增定义：

```go
{
    Name:        "token_stats",
    Description: "查看 MCP 工具使用期间累计节省的 token 统计（按日维度）。基于基线对照法估算。",
    InputSchema: map[string]interface{}{
        "type":                 "object",
        "properties":           map[string]interface{}{},
        "additionalProperties": false,
    },
}
```

### 5.4 入口注入（main.go runMCPMode）

```go
func runMCPMode(cfg config.Config) {
    // ... 现有 indexMgr 创建逻辑 ...

    // 创建 tracker
    var tracker *tokenstats.Tracker
    if cfg.TokenStatsEnabled {
        store := tokenstats.NewStore(cfg.TokenStatsPath)
        baseline := tokenstats.BaselineConfig{
            CodeSearchFileTokens:   cfg.TokenStatsCodeSearchBaseline,
            FileContextBaseline:    cfg.TokenStatsFileContextBaseline,
            SymbolSearchBaseline:   cfg.TokenStatsSymbolSearchBaseline,
            ImpactAnalysisBaseline: cfg.TokenStatsImpactAnalysisBaseline,
        }
        tracker = tokenstats.NewTracker(store, baseline, cfg.TokenStatsCharsPerToken, true, cfg.TokenStatsRetentionDays)
        slog.Info("Token 统计已启用", "path", cfg.TokenStatsPath)
    }

    srv := server.NewMCPServer(cfg, version)
    srv.SetTracker(tracker)
    tools.RegisterTools(srv, cfg, indexMgr, tracker)
    // ...
}
```

## 六、token_stats 输出格式

```
=== Token 节省统计 ===
统计周期: 2026-06-01 ~ 2026-06-29
总调用次数: 142
总输出 token: 89,432
总节省 token: 287,568 (估算)

按工具分组:
工具              调用次数  输出token   节省token   平均耗时
code_search       68       42,180     156,420    320ms
file_context      45       28,500     18,200     5ms
symbol_search     18       12,400     89,200     15ms
impact_analysis   11       6,352      23,748     45ms

近 7 天趋势:
日期        调用次数  节省token
06-23       12       24,100
06-24       18       36,800
06-25       22       41,200
06-26       15       28,500
06-27       20       38,900
06-28       25       52,300
06-29       30       65,768

说明:
- 节省量基于基线对照法估算，仅供参考
- 可通过 TOKEN_STATS_*_BASELINE 调整基线
- 统计保留 90 天，可通过 TOKEN_STATS_RETENTION_DAYS 调整
```

## 七、JSON 持久化结构示例

```json
{
  "version": "1",
  "created_at": "2026-06-01T10:00:00Z",
  "updated_at": "2026-06-29T15:30:00Z",
  "total_calls": 142,
  "total_input_tokens": 1850,
  "total_output_tokens": 89432,
  "total_saved_tokens": 287568,
  "by_tool": {
    "code_search": {
      "call_count": 68,
      "total_input_tokens": 1200,
      "total_output_tokens": 42180,
      "total_saved_tokens": 156420,
      "total_duration_ms": 21760
    }
  },
  "daily": [
    {
      "date": "2026-06-23",
      "call_count": 12,
      "input_tokens": 150,
      "output_tokens": 7200,
      "saved_tokens": 24100
    },
    {
      "date": "2026-06-24",
      "call_count": 18,
      "input_tokens": 220,
      "output_tokens": 11500,
      "saved_tokens": 36800
    }
  ]
}
```

## 八、边界情况

| 场景 | 处理 |
|------|------|
| `TOKEN_STATS_ENABLED=false`（默认） | tracker 为 nil，埋点跳过，`token_stats` 返回"未启用" |
| `index_project` 调用 | skipTracking 过滤，不记录 |
| `token_stats` 自身调用 | skipTracking 过滤，不记录 |
| 工具调用失败（err != nil） | 不记录 |
| `code_search` top_k 未传/非法 | 默认 5 |
| `code_search` top_k > 10 | 截断为 10 |
| `file_context` mode=full | 基线为 0，节省为 0 |
| 输出超过基线 | max(0, ...) 不为负 |
| daily 超过 retentionDays | Record 时裁剪旧条目 |
| 统计文件损坏 | Load 失败重建空快照，warn 日志 |
| 统计文件不存在 | 返回空快照，首次 Record 创建 |
| tracker 为 nil（-index 模式或禁用） | 埋点 `s.tracker != nil` 检查跳过 |

## 九、实施顺序

按依赖关系排序，每步可独立提交：

| 步骤 | 内容 | 涉及文件 |
|------|------|---------|
| 1 | 文件路径收束：改 defaultLocalVectorStorePath、NewIndexStateStore | `config.go`, `index_state.go`, `index_state_test.go` |
| 2 | 部署目录改名：`code-text` → `code-context-mcp` | `build.ps1`, `.gitignore` |
| 3 | 文档批量更新 | `AGENTS.md`, `CLAUDE.md`, `README.md`, `.env.example`, `configs/examples/*.example`, `docs/promotion.md`, `config-dependency-map.md` |
| 4 | tokenstats 包：types + estimator + baseline | `internal/tokenstats/types.go`, `estimator.go`, `baseline.go`, 对应 `_test.go` |
| 5 | tokenstats 包：store + tracker（含 daily 聚合） | `internal/tokenstats/store.go`, `tracker.go`, 对应 `_test.go` |
| 6 | 集成：Config 新增 8 字段 + LoadConfig | `config.go` |
| 7 | 集成：server.go 埋点 + SetTracker | `server.go` |
| 8 | 集成：tools.go 注册 token_stats + tool_defs.go 定义 | `tools.go`, `tool_defs.go` |
| 9 | 集成：main.go 创建 tracker 注入 | `main.go` |
| 10 | 验证：`make vet && make test && make build` | — |

## 十、测试要点

| 测试文件 | 覆盖点 |
|---------|--------|
| `estimator_test.go` | 纯 ASCII / 中文混合 / 空字符串 / charsPerToken=3.5 可配 |
| `baseline_test.go` | 各工具基线计算 / file_context full=0 / mode 未传=summary / top_k=0 或负数→默认5 / top_k=20→截断10 / 节省不为负 |
| `tracker_test.go` | 并发 100 次 Record `-race` / 聚合正确 / daily 按日分桶 / retentionDays 裁剪 / 禁用时空操作 / skipTracking 过滤 |
| `store_test.go` | Load 不存在返空 / Save-Load 一致性 / 原子性（写入中断不损坏） |

现有 `index_state_test.go` 需更新断言路径。
