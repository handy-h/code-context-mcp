package types

import "time"

// IndexState 索引状态
type IndexState struct {
	LastIndexedAt time.Time         `json:"last_indexed_at"`
	Fingerprint   string            `json:"fingerprint"`
	TotalFiles    int               `json:"total_files"`
	TotalChunks   int               `json:"total_chunks"`
	ProjectPath   string            `json:"project_path"`
	FileMtimes    map[string]string `json:"file_mtimes"`
}

// CodeChunk 代码切分块
type CodeChunk struct {
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
}

// SymbolOccurrence 符号出现位置
type SymbolOccurrence struct {
	Symbol  string `json:"symbol"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Type    string `json:"type"`    // definition / reference
	Context string `json:"context"` // 上下文摘要
}

// FileSummary 文件结构摘要
type FileSummary struct {
	File      string     `json:"file"`
	Lines     int        `json:"lines"`
	Language  string     `json:"language"`
	Imports   []string   `json:"imports"`
	Functions []FuncInfo `json:"functions"`
	Types     []TypeInfo `json:"types"`
}

// FuncInfo 函数信息
type FuncInfo struct {
	Name      string `json:"name"`
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
}

// TypeInfo 类型信息
type TypeInfo struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // struct / interface / class
	Line int    `json:"line"`
}

// ImpactResult 影响分析结果
type ImpactResult struct {
	Symbol  string        `json:"symbol"`
	Action  string        `json:"action"`
	Impacts []ImpactItem  `json:"impacts"`
	Summary ImpactSummary `json:"summary"`
}

// ImpactItem 影响条目
type ImpactItem struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Type       string `json:"type"`
	Context    string `json:"context"`
	Suggestion string `json:"suggestion"`
}

// ImpactSummary 影响汇总
type ImpactSummary struct {
	TotalFiles      int            `json:"total_files"`
	TotalReferences int            `json:"total_references"`
	Categories      map[string]int `json:"categories"`
}
