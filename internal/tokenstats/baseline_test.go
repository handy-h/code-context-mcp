package tokenstats

import "testing"

func TestBaseline_CodeSearch(t *testing.T) {
	cfg := BaselineConfig{
		CodeSearchFileTokens:    2000,
		FileContextBaseline:     3000,
		SymbolSearchBaseline:    8000,
		ImpactAnalysisBaseline:  12000,
	}

	tests := []struct {
		name       string
		topK       interface{}
		wantBaseline int
	}{
		{"topK未传", nil, 5 * 2000},              // 默认5
		{"topK=0", 0.0, 5 * 2000},                // 非法值→5
		{"topK负数", -1.0, 5 * 2000},             // 非法值→5
		{"topK=3", 3.0, 3 * 2000},
		{"topK=10", 10.0, 10 * 2000},
		{"topK=20", 20.0, 10 * 2000},             // 截断为10
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{}
			if tt.topK != nil {
				args["top_k"] = tt.topK
			}
			baseline := cfg.calcBaseline("code_search", args)
			if baseline != tt.wantBaseline {
				t.Errorf("topK=%v: baseline=%d, want %d", tt.topK, baseline, tt.wantBaseline)
			}
		})
	}
}

func TestBaseline_FileContext(t *testing.T) {
	cfg := BaselineConfig{
		CodeSearchFileTokens:    2000,
		FileContextBaseline:     3000,
		SymbolSearchBaseline:    8000,
		ImpactAnalysisBaseline:  12000,
	}

	tests := []struct {
		name          string
		mode          interface{}
		wantBaseline  int
	}{
		{"mode未传", nil, 0},       // 默认full，无节省
		{"mode=full", "full", 0},
		{"mode=summary", "summary", 3000},
		{"mode=\"\"", "", 0},       // 空字符串视为full
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{}
			if tt.mode != nil {
				args["mode"] = tt.mode
			}
			baseline := cfg.calcBaseline("file_context", args)
			if baseline != tt.wantBaseline {
				t.Errorf("mode=%v: baseline=%d, want %d", tt.mode, baseline, tt.wantBaseline)
			}
		})
	}
}

func TestBaseline_SymbolSearch(t *testing.T) {
	cfg := BaselineConfig{
		CodeSearchFileTokens:    2000,
		FileContextBaseline:     3000,
		SymbolSearchBaseline:    8000,
		ImpactAnalysisBaseline:  12000,
	}

	baseline := cfg.calcBaseline("symbol_search", nil)
	if baseline != 8000 {
		t.Errorf("SymbolSearch baseline = %d, want 8000", baseline)
	}
}

func TestBaseline_ImpactAnalysis(t *testing.T) {
	cfg := BaselineConfig{
		CodeSearchFileTokens:    2000,
		FileContextBaseline:     3000,
		SymbolSearchBaseline:    8000,
		ImpactAnalysisBaseline:  12000,
	}

	baseline := cfg.calcBaseline("impact_analysis", nil)
	if baseline != 12000 {
		t.Errorf("ImpactAnalysis baseline = %d, want 12000", baseline)
	}
}

func TestBaseline_UnknownTool(t *testing.T) {
	cfg := BaselineConfig{CodeSearchFileTokens: 2000}
	baseline := cfg.calcBaseline("unknown_tool", nil)
	if baseline != 0 {
		t.Errorf("Unknown tool baseline = %d, want 0", baseline)
	}
}

func TestCalculateMetrics_ValidResult_NotNegative(t *testing.T) {
	cfg := BaselineConfig{CodeSearchFileTokens: 2000}
	saved, _ := cfg.CalculateMetrics("code_search", map[string]interface{}{"top_k": 1.0}, 10000, ResultValid)
	if saved != 0 {
		t.Errorf("CalculateMetrics over baseline = %d, want 0", saved)
	}
}

func TestCalculateMetrics_ValidResult_UnderBaseline(t *testing.T) {
	cfg := BaselineConfig{CodeSearchFileTokens: 2000}
	saved, _ := cfg.CalculateMetrics("code_search", map[string]interface{}{"top_k": 1.0}, 500, ResultValid)
	if saved != 1500 {
		t.Errorf("CalculateMetrics under baseline = %d, want 1500", saved)
	}
}

func TestCalculateMetrics_EmptyResult_Wasted(t *testing.T) {
	cfg := BaselineConfig{CodeSearchFileTokens: 2000}
	saved, wasted := cfg.CalculateMetrics("code_search", map[string]interface{}{"top_k": 1.0}, 150, ResultEmpty)
	if saved != 0 {
		t.Errorf("CalculateMetrics empty result saved = %d, want 0", saved)
	}
	if wasted != 150 {
		t.Errorf("CalculateMetrics empty result wasted = %d, want 150", wasted)
	}
}

func TestCalculateMetrics_SystemIssue_Wasted(t *testing.T) {
	cfg := BaselineConfig{CodeSearchFileTokens: 2000}
	saved, wasted := cfg.CalculateMetrics("symbol_search", nil, 80, ResultSystemIssue)
	if saved != 0 {
		t.Errorf("CalculateMetrics system issue saved = %d, want 0", saved)
	}
	if wasted != 80 {
		t.Errorf("CalculateMetrics system issue wasted = %d, want 80", wasted)
	}
}
