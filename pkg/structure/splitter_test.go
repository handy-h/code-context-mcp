package structure

import (
	"strings"
	"testing"

	"github.com/handy-h/code-context-mcp/internal/types"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"main.go", "go"},
		{"app.vue", "vue"},
		{"index.js", "js"},
		{"types.ts", "ts"},
		{"README.md", "md"},
		{"main.py", "py"},
		{"", ""},
		{"style.css", ""},
		{"Makefile", ""},
		{"path/to/file.go", "go"},
		{"FILE.GO", "go"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := DetectLanguage(tt.path); got != tt.want {
				t.Errorf("DetectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestSplitByStructure_EmptyContent(t *testing.T) {
	chunks := SplitByStructure("", "go", "test.go", 800)
	if chunks != nil {
		t.Errorf("SplitByStructure(empty) = %v, want nil", chunks)
	}

	chunks = SplitByStructure("   \n  ", "go", "test.go", 800)
	if chunks != nil {
		t.Errorf("SplitByStructure(whitespace) = %v, want nil", chunks)
	}
}

func TestSplitByStructure_AutoDetect(t *testing.T) {
	content := "package main\n\nfunc main() {}"
	chunks := SplitByStructure(content, "", "main.go", 800)
	if len(chunks) == 0 {
		t.Error("SplitByStructure with auto-detect returned empty")
	}
}

func TestSplitByStructure_FallbackFixed(t *testing.T) {
	content := strings.Repeat("x", 500)
	chunks := SplitByStructure(content, "unknown", "test.xyz", 200)
	if len(chunks) == 0 {
		t.Error("SplitByStructure with unknown lang should fallback to fixed-size")
	}
}

func TestChunkByFixedSize(t *testing.T) {
	content := strings.Repeat("abcdefghij", 100)
	chunks := chunkByFixedSize(content, "test.txt", 200)
	if len(chunks) == 0 {
		t.Error("chunkByFixedSize returned empty")
	}
	for _, c := range chunks {
		if len(strings.TrimSpace(c.Content)) < 10 {
			t.Error("chunk with < 10 non-whitespace chars should be skipped")
		}
	}
}

func TestChunkByFixedSize_SmallChunk(t *testing.T) {
	content := "   \n   "
	chunks := chunkByFixedSize(content, "test.txt", 200)
	if len(chunks) != 0 {
		t.Errorf("small whitespace-only chunks should be skipped, got %d", len(chunks))
	}
}

func TestSplitOversizedChunks_NoSplit(t *testing.T) {
	chunks := []types.CodeChunk{
		{Content: "short content", Metadata: map[string]interface{}{"line_start": 1}},
	}
	result := splitOversizedChunks(chunks, 1000)
	if len(result) != 1 {
		t.Errorf("splitOversizedChunks(short) returned %d chunks, want 1", len(result))
	}
}

func TestSplitOversizedChunks_WithSplit(t *testing.T) {
	longLine := strings.Repeat("x", 200)
	content := longLine + "\n" + longLine + "\n" + longLine
	chunks := []types.CodeChunk{
		{
			Content:  content,
			Metadata: map[string]interface{}{"line_start": 1},
		},
	}
	result := splitOversizedChunks(chunks, 300)
	if len(result) < 2 {
		t.Errorf("splitOversizedChunks should split long chunks, got %d", len(result))
	}
}

func TestSplitOversizedChunks_LineBoundary(t *testing.T) {
	chunks := []types.CodeChunk{
		{
			Content:  "line1\nline2\nline3\nline4",
			Metadata: map[string]interface{}{"line_start": 1},
		},
	}
	result := splitOversizedChunks(chunks, 15)
	for _, c := range result {
		if strings.Contains(c.Content, "\nline2\nline3\n") {
			t.Error("split should respect line boundaries")
		}
	}
}
