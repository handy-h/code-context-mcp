# code-context MCP 服务 — 实施方案

## 1. 任务依赖关系

```
任务1(配置扩展) → 任务2(结构切分器) → 任务4(索引状态存储) → 任务5(索引管理器) → 任务6(服务集成)
                → 任务3(倒排索引)  ──────────────────────┘
任务2 → 任务7(文件摘要)
任务6 → 任务8(symbol_search) → 任务9(impact_analysis)
任务7 → 任务10(file_context摘要)
```

## 2. 实施记录

### 任务 1：配置扩展与数据结构定义 [已完成]

**修改文件：** `config.go`
**新增文件：** `types.go`

- Config 新增 MaxChunkSize、AutoIndex、ProjectPath、IndexStatePath 配置项
- 新增 getEnvBool 辅助函数
- 定义 IndexState、CodeChunk、SymbolOccurrence、FileSummary、FuncInfo、TypeInfo、ImpactResult、ImpactItem、ImpactSummary 结构体

### 任务 2：代码结构切分器实现 [已完成]

**新增文件：** `structure_splitter.go`

- detectLanguage: .go/.vue/.js/.ts/.md/.py 语言检测
- SplitByStructure: 按语法结构切分，超长块二次切分
- Go: 按 func/type struct/type interface/var/const 边界切分
- Vue: 按 template/script/style 块切分，script 内按 JS/TS 切分
- JS/TS: 按 function/class/export/const 边界切分
- Markdown: 按标题切分
- Python: 按 def/class 边界切分
- 无法识别语言降级为固定字符窗口切分

### 任务 3：倒排索引实现 [已完成]

**新增文件：** `inverted_index.go`

- InvertedIndex: 内存倒排索引，sync.RWMutex 并发安全
- BuildFromChunks: 从切分块提取标识符构建索引
- Search: 按符号名搜索，支持 definition/reference/all 过滤
- GetAllOccurrences: 获取符号所有出现位置
- expandQuery: 驼峰/下划线风格转换（DiagnosisNotes ↔ diagnosis_notes）
- RemoveFile: 移除文件相关索引项

### 任务 4：索引状态存储实现 [已完成]

**新增文件：** `index_state.go`

- IndexStateStore: 索引状态 JSON 文件持久化
- Load/Save: 读写索引状态文件
- GetCurrentFingerprint: 优先 git commit hash，降级为文件 mtime SHA256 摘要

### 任务 5：索引管理器实现 [已完成]

**新增文件：** `index_manager.go`
**修改文件：** `indexer.go`, `vectordb.go`

- IndexManager: 索引生命周期管理
- CheckAndAutoIndex: 启动时自动检测索引状态
- TriggerUpdateIfStale: 搜索时过期触发后台增量更新
- IncrementalUpdate: 增量更新（找变更文件→删旧向量→重新索引→更新倒排索引→保存状态）
- BuildIndex: 改用 SplitByStructure 替代 SplitText，metadata 扩展
- VectorDB 新增 HasCollection 和 DeleteByFile 方法

### 任务 6：MCP 服务集成与启动流程修改 [已完成]

**修改文件：** `main.go`, `tools.go`

- runMCPMode: 创建 IndexManager 并启动自动索引
- RegisterTools: 签名增加 indexMgr 参数
- code_search: 增加过期检测逻辑

### 任务 7：文件摘要提取实现 [已完成]

**新增文件：** `file_summary.go`

- ExtractSummary: 提取文件结构摘要
- Go/Vue/JS/TS/MD/Py 六种语言的 imports、functions、types 提取

### 任务 8：symbol_search 工具实现 [已完成]

**修改文件：** `tools.go`

- 新增 symbol_search 工具定义和处理器
- 调用 InvertedIndex.Search，按文件分组格式化输出

### 任务 9：impact_analysis 工具实现 [已完成]

**修改文件：** `tools.go`

- 新增 impact_analysis 工具定义和处理器
- 调用 InvertedIndex.GetAllOccurrences，根据 action 生成 suggestion
- 返回 JSON 格式结果，含 impacts 数组和 summary 汇总

### 任务 10：file_context 摘要模式实现 [已完成]

**修改文件：** `tools.go`

- file_context InputSchema 新增 mode 属性（full/summary）
- mode=summary 调用 ExtractSummary 返回 JSON 格式摘要
- mode=full 保持原有行为

## 3. 文件清单

### 新增文件

| 文件 | 行数 | 功能 |
|------|------|------|
| types.go | ~76 | 核心数据结构定义 |
| structure_splitter.go | ~646 | 代码结构切分器 |
| inverted_index.go | ~299 | 内存倒排索引 |
| index_state.go | ~139 | 索引状态持久化 |
| index_manager.go | ~296 | 索引管理器 |
| file_summary.go | ~284 | 文件摘要提取 |

### 修改文件

| 文件 | 修改内容 |
|------|---------|
| config.go | 新增 4 个配置项 + getEnvBool |
| indexer.go | BuildIndex 改用 SplitByStructure，构建倒排索引 |
| vectordb.go | 新增 HasCollection、DeleteByFile |
| main.go | runMCPMode 创建 IndexManager，runIndexMode 适配新签名 |
| tools.go | RegisterTools 增加 indexMgr；新增 symbol_search、impact_analysis；file_context 增加 mode |
