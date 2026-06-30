package tokenstats

import (
	"fmt"
	"sort"
	"strings"
)

// FormatStats 将统计快照格式化为文本输出
func FormatStats(stats StatsSnapshot) string {
	var b strings.Builder

	b.WriteString("=== Token 节省统计 ===\n")

	// 统计周期
	if len(stats.Daily) > 0 {
		// Daily 已按时间顺序，取首尾
		first := stats.Daily[0].Date
		last := stats.Daily[len(stats.Daily)-1].Date
		fmt.Fprintf(&b, "统计周期: %s ~ %s\n", first, last)
	}

	fmt.Fprintf(&b, "总调用次数: %d\n", stats.TotalCalls)
	fmt.Fprintf(&b, "总输出 token: %s\n", formatNumber(stats.TotalOutputTokens))
	fmt.Fprintf(&b, "总节省 token: %s (估算)\n", formatNumber(stats.TotalSavedTokens))
	fmt.Fprintf(&b, "总浪费 token: %s (空结果消耗)\n", formatNumber(stats.TotalWastedTokens))

	// 按工具分组
	if len(stats.ByTool) > 0 {
		b.WriteString("\n按工具分组:\n")
		b.WriteString("工具              调用次数  输出token   节省token   浪费token   平均耗时\n")

		// 按工具名排序输出
		names := make([]string, 0, len(stats.ByTool))
		for name := range stats.ByTool {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			tool := stats.ByTool[name]
			avgMs := int64(0)
			if tool.CallCount > 0 {
				avgMs = tool.TotalDurationMs / tool.CallCount
			}
			fmt.Fprintf(&b, "%-16s  %8d  %10s  %10s  %10s  %8dms\n",
				name,
				tool.CallCount,
				formatNumber(tool.TotalOutputTokens),
				formatNumber(tool.TotalSavedTokens),
				formatNumber(tool.TotalWastedTokens),
				avgMs,
			)
		}
	}

	// 近 7 天趋势
	if len(stats.Daily) > 0 {
		b.WriteString("\n近 7 天趋势:\n")
		b.WriteString("日期        调用次数  节省token   浪费token\n")

		start := 0
		if len(stats.Daily) > 7 {
			start = len(stats.Daily) - 7
		}
		for _, d := range stats.Daily[start:] {
			// 只取 MM-DD
			shortDate := d.Date
			if len(shortDate) >= 10 {
				shortDate = d.Date[5:10]
			}
			fmt.Fprintf(&b, "%s    %8d  %10s  %10s\n", shortDate, d.CallCount, formatNumber(d.SavedTokens), formatNumber(d.WastedTokens))
		}
	}

	// 说明
	b.WriteString("\n说明:\n")
	b.WriteString("- 节省量基于基线对照法估算，仅供参考\n")
	b.WriteString("- 浪费token：空结果或系统状态问题导致的无效调用消耗的token\n")
	b.WriteString("- 净节省 = 总节省 - 总浪费\n")
	b.WriteString("- 可通过 TOKEN_STATS_*_BASELINE 调整基线\n")
	b.WriteString("- 统计保留 90 天，可通过 TOKEN_STATS_RETENTION_DAYS 调整\n")

	return b.String()
}

func formatNumber(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	offset := len(s) % 3
	if offset == 0 {
		offset = 3
	}
	result = append(result, s[:offset]...)
	for i := offset; i < len(s); i += 3 {
		result = append(result, ',')
		result = append(result, s[i:i+3]...)
	}
	return string(result)
}
