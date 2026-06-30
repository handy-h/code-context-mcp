package tokenstats

import "encoding/json"

// BaselineConfig 各工具的基线参数
type BaselineConfig struct {
	CodeSearchFileTokens    int // 默认 2000
	FileContextBaseline     int // 默认 3000
	SymbolSearchBaseline    int // 默认 8000
	ImpactAnalysisBaseline  int // 默认 12000
}

// CalculateMetrics 根据结果质量计算节省和浪费token
// 有效结果：saved = max(0, baseline - outputTokens), wasted = 0
// 空结果/系统状态：saved = 0, wasted = outputTokens
func (c BaselineConfig) CalculateMetrics(toolName string, args map[string]interface{}, outputTokens int, quality ResultQuality) (saved int, wasted int) {
	switch quality {
	case ResultValid:
		baseline := c.calcBaseline(toolName, args)
		saved = baseline - outputTokens
		if saved < 0 {
			saved = 0
		}
		wasted = 0
	default:
		// ResultEmpty 或 ResultSystemIssue：整个输出算浪费
		saved = 0
		wasted = outputTokens
	}

	return saved, wasted
}

func (c BaselineConfig) calcBaseline(toolName string, args map[string]interface{}) int {
	switch toolName {
	case "code_search":
		topK := extractTopK(args)
		return topK * c.CodeSearchFileTokens
	case "file_context":
		mode, _ := args["mode"].(string)
		if mode == "summary" {
			return c.FileContextBaseline
		}
		return 0 // mode=full（含未传值的默认情况）无节省
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
	var f float64
	switch val := v.(type) {
	case float64:
		f = val
	case int:
		f = float64(val)
	case json.Number:
		if n, err := val.Float64(); err == nil {
			f = n
		} else {
			return 5
		}
	default:
		return 5
	}
	if f <= 0 {
		return 5
	}
	k := int(f)
	if k > 10 {
		return 10
	}
	return k
}
