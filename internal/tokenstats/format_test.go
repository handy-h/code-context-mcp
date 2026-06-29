package tokenstats

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestFormatNumber_Zero(t *testing.T) {
	got := formatNumber(0)
	if got != "0" {
		t.Errorf("formatNumber(0) = %q, want \"0\"", got)
	}
}

func TestFormatNumber_LessThan1000(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{1, "1"},
		{99, "99"},
		{999, "999"},
	}
	for _, tt := range tests {
		got := formatNumber(tt.n)
		if got != tt.want {
			t.Errorf("formatNumber(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestFormatNumber_Thousands(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{1000, "1,000"},
		{9999, "9,999"},
		{10000, "10,000"},
		{100000, "100,000"},
		{1000000, "1,000,000"},
		{1234567890, "1,234,567,890"},
		{math.MaxInt64, "9,223,372,036,854,775,807"},
	}
	for _, tt := range tests {
		got := formatNumber(tt.n)
		if got != tt.want {
			t.Errorf("formatNumber(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestFormatStats_Empty(t *testing.T) {
	stats := StatsSnapshot{
		Version:   "1",
		ByTool:    make(map[string]*ToolStats),
		Daily:     nil,
	}
	result := FormatStats(stats)
	if !strings.Contains(result, "=== Token 节省统计 ===") {
		t.Error("FormatStats should contain title")
	}
	if !strings.Contains(result, "总调用次数: 0") {
		t.Error("FormatStats should show zero calls")
	}
	// 空无数据时不展示按工具/趋势表格
	if strings.Contains(result, "按工具分组") {
		t.Error("FormatStats should not show tool table when ByTool is empty")
	}
	if strings.Contains(result, "近 7 天趋势") {
		t.Error("FormatStats should not show trend when Daily is empty")
	}
}

func TestFormatStats_WithData(t *testing.T) {
	stats := StatsSnapshot{
		Version:           "1",
		TotalCalls:        10,
		TotalOutputTokens: 5000,
		TotalSavedTokens:  3000,
		ByTool: map[string]*ToolStats{
			"code_search": {
				CallCount:         8,
				TotalOutputTokens: 4000,
				TotalSavedTokens:  2500,
				TotalDurationMs:   800,
			},
			"symbol_search": {
				CallCount:         2,
				TotalOutputTokens: 1000,
				TotalSavedTokens:  500,
				TotalDurationMs:   200,
			},
		},
		Daily: []DailyStats{
			{Date: "2026-06-28", CallCount: 4, OutputTokens: 2000, SavedTokens: 1200},
			{Date: "2026-06-29", CallCount: 6, OutputTokens: 3000, SavedTokens: 1800},
		},
	}
	result := FormatStats(stats)

	// 关键字段存在
	checks := []string{
		"=== Token 节省统计 ===",
		"总调用次数: 10",
		"总输出 token: 5,000",
		"总节省 token: 3,000",
		"按工具分组",
		"code_search",
		"symbol_search",
		"近 7 天趋势",
		"06-28",
		"06-29",
	}
	for _, c := range checks {
		if !strings.Contains(result, c) {
			t.Errorf("FormatStats missing %q in output:\n%s", c, result)
		}
	}
}

func TestFormatStats_DailyTruncation(t *testing.T) {
	// 超过 7 天时只显示最近 7 天
	var daily []DailyStats
	for i := 0; i < 10; i++ {
		daily = append(daily, DailyStats{
			Date:        fmt.Sprintf("2026-06-%02d", 20+i),
			CallCount:   int64(i + 1),
			SavedTokens: int64(i * 100),
		})
	}
	stats := StatsSnapshot{
		Version: "1",
		ByTool:  make(map[string]*ToolStats),
		Daily:   daily,
	}
	result := FormatStats(stats)

	// 找出"近 7 天趋势"之后的文本
	trendIdx := strings.Index(result, "近 7 天趋势")
	if trendIdx < 0 {
		t.Fatal("should contain trend section")
	}
	trendSection := result[trendIdx:]

	// 前3天（06-20, 06-21, 06-22）不应出现在趋势区域
	for _, excluded := range []string{"06-20", "06-21", "06-22"} {
		if strings.Contains(trendSection, excluded) {
			t.Errorf("trend section should NOT contain %s (before 7-day cutoff)", excluded)
		}
	}
	// 最近7天（06-23 ~ 06-29）应出现
	for _, included := range []string{"06-28", "06-29"} {
		if !strings.Contains(trendSection, included) {
			t.Errorf("trend section should contain %s", included)
		}
	}
}

func TestFormatNumber_MaxInt64(t *testing.T) {
	got := formatNumber(math.MaxInt64)
	if got == "" {
		t.Error("formatNumber(MaxInt64) should not be empty")
	}
	if !strings.Contains(got, ",") {
		t.Errorf("formatNumber(MaxInt64) = %q, should contain commas", got)
	}
}
