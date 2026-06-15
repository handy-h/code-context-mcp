package tools

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/handy-h/code-context-mcp/internal/config"
	"github.com/handy-h/code-context-mcp/internal/indexer"
	"github.com/handy-h/code-context-mcp/internal/types"
)

func TestParseIntArg_Float64(t *testing.T) {
	args := map[string]interface{}{"top_k": float64(10)}
	if got := parseIntArg(args, "top_k", 5); got != 10 {
		t.Errorf("parseIntArg(float64) = %d, want 10", got)
	}
}

func TestParseIntArg_JSONNumber(t *testing.T) {
	args := map[string]interface{}{"top_k": json.Number("15")}
	if got := parseIntArg(args, "top_k", 5); got != 15 {
		t.Errorf("parseIntArg(json.Number) = %d, want 15", got)
	}
}

func TestParseIntArg_String(t *testing.T) {
	args := map[string]interface{}{"top_k": "20"}
	if got := parseIntArg(args, "top_k", 5); got != 20 {
		t.Errorf("parseIntArg(string) = %d, want 20", got)
	}
}

func TestParseIntArg_Missing(t *testing.T) {
	args := map[string]interface{}{}
	if got := parseIntArg(args, "top_k", 5); got != 5 {
		t.Errorf("parseIntArg(missing) = %d, want 5", got)
	}
}

func TestParseIntArg_InvalidString(t *testing.T) {
	args := map[string]interface{}{"top_k": "abc"}
	if got := parseIntArg(args, "top_k", 5); got != 5 {
		t.Errorf("parseIntArg(invalid) = %d, want 5", got)
	}
}

func TestParseIntArg_Default(t *testing.T) {
	args := map[string]interface{}{"other": 99}
	if got := parseIntArg(args, "top_k", 7); got != 7 {
		t.Errorf("parseIntArg(default) = %d, want 7", got)
	}
}

func TestGenerateSuggestion_Delete(t *testing.T) {
	tests := []struct {
		occType string
		want    string
	}{
		{"definition", "删除该定义"},
		{"reference", "删除该引用"},
	}
	for _, tt := range tests {
		t.Run(tt.occType, func(t *testing.T) {
			got := generateSuggestion("delete", "MyFunc", "", tt.occType)
			if got != tt.want {
				t.Errorf("generateSuggestion(delete, %s) = %q, want %q", tt.occType, got, tt.want)
			}
		})
	}
}

func TestGenerateSuggestion_Rename(t *testing.T) {
	got := generateSuggestion("rename", "OldName", "NewName", "reference")
	want := "将 OldName 替换为 NewName"
	if got != want {
		t.Errorf("generateSuggestion(rename) = %q, want %q", got, want)
	}
}

func TestGenerateSuggestion_Modify(t *testing.T) {
	tests := []struct {
		occType string
		want    string
	}{
		{"definition", "更新定义以匹配新签名"},
		{"reference", "更新调用以匹配新签名"},
	}
	for _, tt := range tests {
		t.Run(tt.occType, func(t *testing.T) {
			got := generateSuggestion("modify", "MyFunc", "", tt.occType)
			if got != tt.want {
				t.Errorf("generateSuggestion(modify, %s) = %q, want %q", tt.occType, got, tt.want)
			}
		})
	}
}

func TestGenerateSuggestion_Unknown(t *testing.T) {
	got := generateSuggestion("unknown", "MyFunc", "", "reference")
	if got != "" {
		t.Errorf("generateSuggestion(unknown) = %q, want empty", got)
	}
}

func TestCategorizeImpact_Definition(t *testing.T) {
	if got := categorizeImpact("definition", "func MyFunc()"); got != "definition" {
		t.Errorf("categorizeImpact(definition) = %q, want definition", got)
	}
}

func TestCategorizeImpact_Reference(t *testing.T) {
	if got := categorizeImpact("reference", "MyFunc()"); got != "reference" {
		t.Errorf("categorizeImpact(reference) = %q, want reference", got)
	}
}

func TestCategorizeImpact_JSONTag(t *testing.T) {
	ctx := "Name string `json:\"name\"`"
	if got := categorizeImpact("reference", ctx); got != "api_payload" {
		t.Errorf("categorizeImpact(json tag) = %q, want api_payload", got)
	}
}

func TestContainsJSONTag_Found(t *testing.T) {
	s := "Name string `json:\"name\"`"
	if !containsJSONTag(s) {
		t.Error("containsJSONTag should find json:\" tag")
	}
}

func TestContainsJSONTag_NotFound(t *testing.T) {
	if containsJSONTag("just a normal string") {
		t.Error("containsJSONTag should not find tag in normal string")
	}
}

func TestContainsJSONTag_ShortString(t *testing.T) {
	if containsJSONTag("abc") {
		t.Error("containsJSONTag should return false for short string")
	}
}

func TestHandleSymbolSearch_Empty(t *testing.T) {
	mgr := newIndexManagerForTest(t)
	handler := handleSymbolSearch(mgr)
	result, err := handler(map[string]interface{}{"query": "MyFunc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "符号索引尚未构建，请先索引项目。" {
		t.Errorf("empty index result = %q", result)
	}
}

func TestHandleSymbolSearch_Results(t *testing.T) {
	mgr := newIndexManagerForTest(t)
	chunks := []types.CodeChunk{
		{Content: "func MyFunc() {}", Metadata: map[string]interface{}{"symbol": "MyFunc", "line_start": 1}},
	}
	mgr.GetInvertedIndex().BuildFromChunks(chunks, "test.go")
	handler := handleSymbolSearch(mgr)
	result, err := handler(map[string]interface{}{"query": "MyFunc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "符号索引尚未构建，请先索引项目。" {
		t.Error("should find results, got 'not built' message")
	}
}

func TestHandleSymbolSearch_NilManager(t *testing.T) {
	handler := handleSymbolSearch(nil)
	result, err := handler(map[string]interface{}{"query": "MyFunc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "符号索引尚未构建，请先索引项目。" {
		t.Errorf("nil manager result = %q", result)
	}
}

func TestHandleImpactAnalysis_Empty(t *testing.T) {
	mgr := newIndexManagerForTest(t)
	handler := handleImpactAnalysis(mgr)
	result, err := handler(map[string]interface{}{"symbol": "MyFunc", "action": "delete"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "符号索引尚未构建，请先索引项目。" {
		t.Errorf("empty index result = %q", result)
	}
}

func TestHandleImpactAnalysis_Delete(t *testing.T) {
	mgr := newIndexManagerForTest(t)
	chunks := []types.CodeChunk{
		{Content: "func MyFunc() {}", Metadata: map[string]interface{}{"symbol": "MyFunc", "line_start": 1}},
	}
	mgr.GetInvertedIndex().BuildFromChunks(chunks, "test.go")
	handler := handleImpactAnalysis(mgr)
	result, err := handler(map[string]interface{}{"symbol": "MyFunc", "action": "delete"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "符号索引尚未构建，请先索引项目。" {
		t.Error("should find results")
	}
	if result == "" {
		t.Error("result should not be empty")
	}
}

func TestHandleImpactAnalysis_Rename_NoNewName(t *testing.T) {
	mgr := newIndexManagerForTest(t)
	chunks := []types.CodeChunk{
		{Content: "func MyFunc() {}", Metadata: map[string]interface{}{"symbol": "MyFunc", "line_start": 1}},
	}
	mgr.GetInvertedIndex().BuildFromChunks(chunks, "test.go")
	handler := handleImpactAnalysis(mgr)
	_, err := handler(map[string]interface{}{"symbol": "MyFunc", "action": "rename"})
	if err == nil {
		t.Error("rename without new_name should return error")
	}
}

func TestHandleCodeSearch_EmptyQuery(t *testing.T) {
	handler := handleCodeSearch(config.Config{SearchTimeout: 5 * time.Second}, nil, nil)
	_, err := handler(map[string]interface{}{"query": ""})
	if err == nil {
		t.Error("empty query should return error")
	}
}

func TestHandleFileContext_EmptyPath(t *testing.T) {
	handler := handleFileContext(config.Config{})
	_, err := handler(map[string]interface{}{"file_path": ""})
	if err == nil {
		t.Error("empty file_path should return error")
	}
}

func newIndexManagerForTest(t *testing.T) *indexer.IndexManager {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{
		ScanExtensions: []string{".go"},
		IndexStatePath: dir + "/state.json",
	}
	return indexer.NewIndexManager(cfg, dir)
}
