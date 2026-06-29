package tokenstats

import "time"

// ToolStats 单工具总量聚合
type ToolStats struct {
	CallCount         int64 `json:"call_count"`
	TotalOutputTokens int64 `json:"total_output_tokens"`
	TotalSavedTokens  int64 `json:"total_saved_tokens"`
	TotalDurationMs   int64 `json:"total_duration_ms"`
}

// DailyStats 单日聚合统计
type DailyStats struct {
	Date         string `json:"date"`
	CallCount    int64  `json:"call_count"`
	OutputTokens int64  `json:"output_tokens"`
	SavedTokens  int64  `json:"saved_tokens"`
}

// StatsSnapshot 全量快照（持久化结构）
type StatsSnapshot struct {
	Version           string                `json:"version"`
	CreatedAt         time.Time             `json:"created_at"`
	UpdatedAt         time.Time             `json:"updated_at"`
	TotalCalls        int64                 `json:"total_calls"`
	TotalOutputTokens int64                 `json:"total_output_tokens"`
	TotalSavedTokens  int64                 `json:"total_saved_tokens"`
	ByTool            map[string]*ToolStats `json:"by_tool"`
	Daily             []DailyStats          `json:"daily"`
}

// ToolCallRecord 单次调用记录（不持久化，传给 Tracker.Record）
type ToolCallRecord struct {
	ToolName   string
	Args       map[string]interface{}
	OutputText string
	DurationMs int64
	Timestamp  time.Time
}
