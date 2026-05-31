package search

import (
	"testing"

	"github.com/handy-h/code-context-mcp/internal/types"
)

func TestExpandQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"camelCase", "DiagnosisNotes", []string{"DiagnosisNotes", "diagnosis_notes"}},
		{"snake_case", "diagnosis_notes", []string{"diagnosis_notes", "DiagnosisNotes"}},
		{"single_word", "config", []string{"config", "Config"}},
		{"already_snake", "load_config", []string{"load_config", "LoadConfig"}},
		{"empty", "", []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandQuery(tt.query)
			if len(got) != len(tt.want) {
				t.Fatalf("expandQuery(%q) = %v, want %v", tt.query, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("expandQuery(%q)[%d] = %q, want %q", tt.query, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCamelToSnake(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"DiagnosisNotes", "diagnosis_notes"},
		{"LoadConfig", "load_config"},
		{"HTTPServer", "h_t_t_p_server"},
		{"config", "config"},
		{"A", "a"},
		{"", ""},
		{"ABC", "a_b_c"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := camelToSnake(tt.input); got != tt.want {
				t.Errorf("camelToSnake(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSnakeToCamel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"diagnosis_notes", "DiagnosisNotes"},
		{"load_config", "LoadConfig"},
		{"config", "Config"},
		{"", ""},
		{"a_b_c", "ABC"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := snakeToCamel(tt.input); got != tt.want {
				t.Errorf("snakeToCamel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractContext(t *testing.T) {
	lines := []string{"line0", "line1", "line2", "line3", "line4"}

	tests := []struct {
		name      string
		targetIdx int
		n         int
		wantLines int
	}{
		{"normal", 2, 2, 5},
		{"at_start", 0, 2, 3},
		{"at_end", 4, 2, 3},
		{"single_line", 2, 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := extractContext(lines, tt.targetIdx, tt.n)
			gotLines := len(splitLines(ctx))
			if gotLines != tt.wantLines {
				t.Errorf("extractContext(lines, %d, %d) returned %d lines, want %d", tt.targetIdx, tt.n, gotLines, tt.wantLines)
			}
		})
	}
}

func TestExtractContext_Truncation(t *testing.T) {
	longLine := make([]byte, 300)
	for i := range longLine {
		longLine[i] = 'x'
	}
	lines := []string{string(longLine)}
	ctx := extractContext(lines, 0, 2)
	if len(ctx) > 203 {
		t.Errorf("extractContext should truncate to 200 chars, got %d", len(ctx))
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}

func TestIsKeyword(t *testing.T) {
	positive := []string{"func", "var", "const", "return", "if", "for", "class", "def", "self"}
	negative := []string{"LoadConfig", "myVar", "handleSearch", "diagnosisNotes", "hello"}

	for _, kw := range positive {
		if !isKeyword(kw) {
			t.Errorf("isKeyword(%q) = false, want true", kw)
		}
	}
	for _, id := range negative {
		if isKeyword(id) {
			t.Errorf("isKeyword(%q) = true, want false", id)
		}
	}
}

func TestNewInvertedIndex(t *testing.T) {
	ii := NewInvertedIndex()
	if ii == nil {
		t.Fatal("NewInvertedIndex() returned nil")
	}
	if ii.Size() != 0 {
		t.Errorf("new index Size() = %d, want 0", ii.Size())
	}
}

func TestBuildFromChunks_SingleFile(t *testing.T) {
	ii := NewInvertedIndex()
	chunks := []types.CodeChunk{
		{
			Content: "func LoadConfig() {\n    cfg := loadFromDisk()\n}",
			Metadata: map[string]interface{}{
				"symbol":     "LoadConfig",
				"line_start": 1,
				"type":       "definition",
			},
		},
	}
	ii.BuildFromChunks(chunks, "config.go")

	occ := ii.GetAllOccurrences("LoadConfig")
	if len(occ) == 0 {
		t.Fatal("GetAllOccurrences(LoadConfig) returned empty")
	}
	if occ[0].Type != "definition" {
		t.Errorf("LoadConfig occurrence type = %q, want definition", occ[0].Type)
	}
	if occ[0].File != "config.go" {
		t.Errorf("LoadConfig file = %q, want config.go", occ[0].File)
	}
}

func TestBuildFromChunks_DefinitionVsReference(t *testing.T) {
	ii := NewInvertedIndex()
	chunks := []types.CodeChunk{
		{
			Content: "func LoadConfig() {\n    Save()\n}",
			Metadata: map[string]interface{}{
				"symbol":     "LoadConfig",
				"line_start": 1,
			},
		},
		{
			Content: "func Save() {\n    LoadConfig()\n}",
			Metadata: map[string]interface{}{
				"symbol":     "Save",
				"line_start": 10,
			},
		},
	}
	ii.BuildFromChunks(chunks, "app.go")

	loadOcc := ii.GetAllOccurrences("LoadConfig")
	defCount := 0
	refCount := 0
	for _, occ := range loadOcc {
		if occ.Type == "definition" {
			defCount++
		} else {
			refCount++
		}
	}
	if defCount != 1 {
		t.Errorf("LoadConfig definitions = %d, want 1", defCount)
	}
	if refCount != 1 {
		t.Errorf("LoadConfig references = %d, want 1", refCount)
	}
}

func TestBuildFromChunks_RebuildSameFile(t *testing.T) {
	ii := NewInvertedIndex()
	chunks1 := []types.CodeChunk{
		{Content: "func OldFunc() {}", Metadata: map[string]interface{}{"symbol": "OldFunc"}},
	}
	ii.BuildFromChunks(chunks1, "file.go")

	chunks2 := []types.CodeChunk{
		{Content: "func NewFunc() {}", Metadata: map[string]interface{}{"symbol": "NewFunc"}},
	}
	ii.BuildFromChunks(chunks2, "file.go")

	if len(ii.GetAllOccurrences("OldFunc")) != 0 {
		t.Error("OldFunc should be removed after rebuild")
	}
	if len(ii.GetAllOccurrences("NewFunc")) == 0 {
		t.Error("NewFunc should exist after rebuild")
	}
}

func TestSearch_ExactMatch(t *testing.T) {
	ii := NewInvertedIndex()
	chunks := []types.CodeChunk{
		{Content: "func LoadConfig() {}", Metadata: map[string]interface{}{"symbol": "LoadConfig", "line_start": 1}},
	}
	ii.BuildFromChunks(chunks, "config.go")

	results := ii.Search("LoadConfig", "all", 10)
	if len(results) == 0 {
		t.Fatal("Search returned empty for exact match")
	}
}

func TestSearch_CamelCaseExpansion(t *testing.T) {
	ii := NewInvertedIndex()
	chunks := []types.CodeChunk{
		{Content: "func load_config() {}", Metadata: map[string]interface{}{"symbol": "load_config", "line_start": 1}},
	}
	ii.BuildFromChunks(chunks, "config.go")

	results := ii.Search("load_config", "all", 10)
	if len(results) == 0 {
		t.Fatal("Search should find load_config via snake_case query")
	}

	results2 := ii.Search("LoadConfig", "all", 10)
	if len(results2) == 0 {
		t.Fatal("Search should find load_config via camelCase query (LoadConfig)")
	}
}

func TestSearch_TypeFilter(t *testing.T) {
	ii := NewInvertedIndex()
	chunks := []types.CodeChunk{
		{
			Content:  "func LoadConfig() {\n    Save()\n}",
			Metadata: map[string]interface{}{"symbol": "LoadConfig", "line_start": 1},
		},
		{
			Content:  "func Save() {\n    LoadConfig()\n}",
			Metadata: map[string]interface{}{"symbol": "Save", "line_start": 10},
		},
	}
	ii.BuildFromChunks(chunks, "app.go")

	defs := ii.Search("LoadConfig", "definition", 10)
	for _, d := range defs {
		if d.Type != "definition" {
			t.Errorf("definition filter returned type %q", d.Type)
		}
	}

	refs := ii.Search("LoadConfig", "reference", 10)
	for _, r := range refs {
		if r.Type != "reference" {
			t.Errorf("reference filter returned type %q", r.Type)
		}
	}
}

func TestSearch_TopK(t *testing.T) {
	ii := NewInvertedIndex()
	for i := 0; i < 20; i++ {
		chunks := []types.CodeChunk{
			{Content: "func MyFunc() {}", Metadata: map[string]interface{}{"symbol": "MyFunc", "line_start": i*10 + 1}},
		}
		ii.BuildFromChunks(chunks, "file"+string(rune('a'+i))+".go")
	}

	results := ii.Search("MyFunc", "all", 5)
	if len(results) > 5 {
		t.Errorf("Search topK=5 returned %d results", len(results))
	}
}

func TestSearch_EmptyIndex(t *testing.T) {
	ii := NewInvertedIndex()
	results := ii.Search("anything", "all", 10)
	if len(results) != 0 {
		t.Errorf("Search on empty index returned %d results", len(results))
	}
}

func TestGetAllOccurrences(t *testing.T) {
	ii := NewInvertedIndex()
	chunks := []types.CodeChunk{
		{Content: "func LoadConfig() {}", Metadata: map[string]interface{}{"symbol": "LoadConfig", "line_start": 1}},
	}
	ii.BuildFromChunks(chunks, "a.go")
	ii.BuildFromChunks(chunks, "b.go")

	occ := ii.GetAllOccurrences("LoadConfig")
	if len(occ) < 2 {
		t.Errorf("GetAllOccurrences returned %d, want >= 2", len(occ))
	}
}

func TestRemoveFile(t *testing.T) {
	ii := NewInvertedIndex()
	chunks := []types.CodeChunk{
		{Content: "func MyFunc() {}", Metadata: map[string]interface{}{"symbol": "MyFunc", "line_start": 1}},
	}
	ii.BuildFromChunks(chunks, "a.go")
	ii.BuildFromChunks(chunks, "b.go")

	ii.RemoveFile("a.go")

	occ := ii.GetAllOccurrences("MyFunc")
	for _, o := range occ {
		if o.File == "a.go" {
			t.Error("occurrences from a.go should be removed")
		}
	}
}

func TestRemoveFile_NonExistent(t *testing.T) {
	ii := NewInvertedIndex()
	ii.RemoveFile("nonexistent.go")
}

func TestSize(t *testing.T) {
	ii := NewInvertedIndex()
	if ii.Size() != 0 {
		t.Errorf("empty Size() = %d, want 0", ii.Size())
	}

	chunks := []types.CodeChunk{
		{Content: "func Foo() {\n    Bar()\n}", Metadata: map[string]interface{}{"symbol": "Foo", "line_start": 1}},
		{Content: "func Bar() {}", Metadata: map[string]interface{}{"symbol": "Bar", "line_start": 10}},
	}
	ii.BuildFromChunks(chunks, "app.go")

	if ii.Size() < 2 {
		t.Errorf("Size() = %d, want >= 2", ii.Size())
	}
}

func TestBuildFromChunks_SkipsKeywords(t *testing.T) {
	ii := NewInvertedIndex()
	chunks := []types.CodeChunk{
		{Content: "func if else return", Metadata: map[string]interface{}{"symbol": "TestFunc", "line_start": 1}},
	}
	ii.BuildFromChunks(chunks, "test.go")

	for _, kw := range []string{"func", "if", "else", "return"} {
		if len(ii.GetAllOccurrences(kw)) > 0 {
			t.Errorf("keyword %q should not be indexed", kw)
		}
	}
}

func TestBuildFromChunks_SkipsShortIdentifiers(t *testing.T) {
	ii := NewInvertedIndex()
	chunks := []types.CodeChunk{
		{Content: "x := a + b", Metadata: map[string]interface{}{"symbol": "Calc", "line_start": 1}},
	}
	ii.BuildFromChunks(chunks, "test.go")

	for _, id := range []string{"x", "a", "b"} {
		if len(ii.GetAllOccurrences(id)) > 0 {
			t.Errorf("short identifier %q (len<2) should not be indexed", id)
		}
	}
}

func TestBuildFromChunks_LineNumbers(t *testing.T) {
	ii := NewInvertedIndex()
	chunks := []types.CodeChunk{
		{
			Content: "func MyFunc() {\n    DoSomething()\n}",
			Metadata: map[string]interface{}{
				"symbol":     "MyFunc",
				"line_start": 5,
			},
		},
	}
	ii.BuildFromChunks(chunks, "test.go")

	occ := ii.GetAllOccurrences("MyFunc")
	if len(occ) == 0 {
		t.Fatal("MyFunc not found")
	}
	if occ[0].Line != 5 {
		t.Errorf("MyFunc line = %d, want 5", occ[0].Line)
	}
}
