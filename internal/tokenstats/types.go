package tokenstats

import "time"

// ToolStats 单工具总量聚合
type ToolStats struct {
	CallCount         int64 `json:"call_count"`
	TotalOutputTokens int64 `json:"total_output_tokens"`
	TotalSavedTokens  int64 `json:"total_saved_tokens"`
	TotalWastedTokens int64 `json:"total_wasted_tokens"`
	TotalDurationMs   int64 `json:"total_duration_ms"`
}

// DailyStats 单日聚合统计
type DailyStats struct {
	Date         string `json:"date"`
	CallCount    int64  `json:"call_count"`
	OutputTokens int64  `json:"output_tokens"`
	SavedTokens  int64  `json:"saved_tokens"`
	WastedTokens int64  `json:"wasted_tokens"`
}

// StatsSnapshot 全量快照（持久化结构）
type StatsSnapshot struct {
	Version           string                `json:"version"`
	CreatedAt         time.Time             `json:"created_at"`
	UpdatedAt         time.Time             `json:"updated_at"`
	TotalCalls        int64                 `json:"total_calls"`
	TotalOutputTokens int64                 `json:"total_output_tokens"`
	TotalSavedTokens  int64                 `json:"total_saved_tokens"`
	TotalWastedTokens int64                 `json:"total_wasted_tokens"`
	ByTool            map[string]*ToolStats `json:"by_tool"`
	Daily             []DailyStats          `json:"daily"`
}

// ResultQuality 结果质量枚举
type ResultQuality string

const (
	ResultValid       ResultQuality = "valid"
	ResultEmpty       ResultQuality = "empty"
	ResultSystemIssue ResultQuality = "system"
)

// 结果质量判断相关的文本常量
// 这些文本与 internal/tools/tools.go 中的错误提示保持一致
const (
	// 系统状态问题文本（索引未构建）
	MsgIndexNotBuilt1 = "符号索引尚未构建"
	MsgIndexNotBuilt2 = "请先索引项目"
	MsgIndexNotBuilt3 = "使用 index_project 工具"

	// 空结果文本
	MsgSymbolNotFound   = "未找到符号"
	MsgNoOccurrences    = "任何出现位置"
	MsgNoRelatedCode    = "未找到相关代码"
)

// ToolCallRecord 单次调用记录（不持久化，传给 Tracker.Record）
type ToolCallRecord struct {
	ToolName      string
	Args          map[string]interface{}
	OutputText    string
	DurationMs    int64
	Timestamp     time.Time
	ResultQuality ResultQuality
}
