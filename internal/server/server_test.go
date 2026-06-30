package server

import (
	"testing"

	"github.com/handy-h/code-context-mcp/internal/tokenstats"
)

func TestJudgeResultQuality_SystemIssue(t *testing.T) {
	tests := []struct {
		name       string
		resultText string
		want       tokenstats.ResultQuality
	}{
		{
			name:       "索引未构建提示1",
			resultText: "符号索引尚未构建，请先索引项目。",
			want:       tokenstats.ResultSystemIssue,
		},
		{
			name:       "索引未构建提示2",
			resultText: "请先索引项目后再查询。",
			want:       tokenstats.ResultSystemIssue,
		},
		{
			name:       "索引未构建提示3",
			resultText: "使用 index_project 工具索引项目。",
			want:       tokenstats.ResultSystemIssue,
		},
		{
			name:       "组合提示",
			resultText: "未找到相关代码。可能项目尚未索引，请先使用 index_project 工具。",
			want:       tokenstats.ResultSystemIssue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := judgeResultQuality("symbol_search", tt.resultText)
			if got != tt.want {
				t.Errorf("judgeResultQuality(%q) = %v, want %v", tt.resultText, got, tt.want)
			}
		})
	}
}

func TestJudgeResultQuality_EmptyResult(t *testing.T) {
	tests := []struct {
		name       string
		toolName   string
		resultText string
		want       tokenstats.ResultQuality
	}{
		{
			name:       "symbol_search未找到符号",
			toolName:   "symbol_search",
			resultText: "未找到符号 \"foo\" 的匹配。",
			want:       tokenstats.ResultEmpty,
		},
		{
			name:       "impact_analysis未找到符号",
			toolName:   "impact_analysis",
			resultText: "未找到符号 \"bar\" 的任何出现位置。",
			want:       tokenstats.ResultEmpty,
		},
		{
			name:       "impact_analysis任何出现位置",
			toolName:   "impact_analysis",
			resultText: "未找到任何出现位置。",
			want:       tokenstats.ResultEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := judgeResultQuality(tt.toolName, tt.resultText)
			if got != tt.want {
				t.Errorf("judgeResultQuality(%q, %q) = %v, want %v", tt.toolName, tt.resultText, got, tt.want)
			}
		})
	}
}

func TestJudgeResultQuality_ValidResult(t *testing.T) {
	tests := []struct {
		name       string
		toolName   string
		resultText string
		want       tokenstats.ResultQuality
	}{
		{
			name:       "symbol_search找到结果",
			toolName:   "symbol_search",
			resultText: "找到符号 \"foo\" 的匹配：\n- 文件1.go:10\n- 文件2.go:20",
			want:       tokenstats.ResultValid,
		},
		{
			name:       "impact_analysis找到结果",
			toolName:   "impact_analysis",
			resultText: "符号 \"bar\" 的影响分析：\n- 调用点1\n- 调用点2",
			want:       tokenstats.ResultValid,
		},
		{
			name:       "code_search正常结果",
			toolName:   "code_search",
			resultText: "搜索结果：\n- 文件1.go\n- 文件2.go",
			want:       tokenstats.ResultValid,
		},
		{
			name:       "空文本",
			toolName:   "symbol_search",
			resultText: "",
			want:       tokenstats.ResultValid,
		},
		{
			name:       "其他工具空结果",
			toolName:   "code_search",
			resultText: "未找到符号",
			want:       tokenstats.ResultValid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := judgeResultQuality(tt.toolName, tt.resultText)
			if got != tt.want {
				t.Errorf("judgeResultQuality(%q, %q) = %v, want %v", tt.toolName, tt.resultText, got, tt.want)
			}
		})
	}
}
