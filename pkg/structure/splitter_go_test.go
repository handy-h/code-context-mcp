package structure

import (
	"testing"
)

func TestChunkGo_SimpleFunc(t *testing.T) {
	content := `package main

func Hello() {
    fmt.Println("hello")
}`
	chunks := chunkGo(content, "main.go")
	if len(chunks) == 0 {
		t.Fatal("chunkGo returned empty for simple func")
	}
	found := false
	for _, c := range chunks {
		if sym, ok := c.Metadata["symbol"].(string); ok && sym == "Hello" {
			found = true
			if c.Metadata["type"] != "function" {
				t.Errorf("Hello chunk type = %v, want function", c.Metadata["type"])
			}
		}
	}
	if !found {
		t.Error("Hello function not found in chunks")
	}
}

func TestChunkGo_Method(t *testing.T) {
	content := `package main

func (s *Server) Run() error {
    return nil
}`
	chunks := chunkGo(content, "server.go")
	if len(chunks) == 0 {
		t.Fatal("chunkGo returned empty for method")
	}
	found := false
	for _, c := range chunks {
		if sym, ok := c.Metadata["symbol"].(string); ok && sym == "Run" {
			found = true
		}
	}
	if !found {
		t.Error("Run method not found in chunks")
	}
}

func TestChunkGo_Struct(t *testing.T) {
	content := `package main

type Config struct {
    Name string
    Port int
}`
	chunks := chunkGo(content, "config.go")
	found := false
	for _, c := range chunks {
		if sym, ok := c.Metadata["symbol"].(string); ok && sym == "Config" {
			found = true
			if c.Metadata["type"] != "struct" {
				t.Errorf("Config chunk type = %v, want struct", c.Metadata["type"])
			}
		}
	}
	if !found {
		t.Error("Config struct not found in chunks")
	}
}

func TestChunkGo_Interface(t *testing.T) {
	content := `package main

type Writer interface {
    Write([]byte) (int, error)
}`
	chunks := chunkGo(content, "io.go")
	found := false
	for _, c := range chunks {
		if sym, ok := c.Metadata["symbol"].(string); ok && sym == "Writer" {
			found = true
		}
	}
	if !found {
		t.Error("Writer interface not found in chunks")
	}
}

func TestChunkGo_MixedContent(t *testing.T) {
	content := `package main

import "fmt"

type Server struct {
    addr string
}

func NewServer(addr string) *Server {
    return &Server{addr: addr}
}

func (s *Server) Start() error {
    return nil
}`
	chunks := chunkGo(content, "server.go")
	if len(chunks) < 3 {
		t.Errorf("chunkGo should find >= 3 chunks (Server, NewServer, Start), got %d", len(chunks))
	}
}

func TestChunkGo_WithHeader(t *testing.T) {
	content := `package main

import "fmt"

func Hello() {
    fmt.Println("hello")
}`
	chunks := chunkGo(content, "main.go")
	if len(chunks) == 0 {
		t.Fatal("chunkGo returned empty")
	}
	for _, c := range chunks {
		if c.Metadata["type"] == "header" {
			if c.Metadata["symbol"] != "header" {
				t.Errorf("header chunk symbol = %v, want header", c.Metadata["symbol"])
			}
		}
	}
}

func TestChunkGo_NoBoundary(t *testing.T) {
	content := `// just a comment
x := 1`
	chunks := chunkGo(content, "test.go")
	if len(chunks) != 0 {
		t.Errorf("chunkGo(no boundary) = %d chunks, want 0", len(chunks))
	}
}

func TestIsContinuation(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		idx   int
		want  bool
	}{
		{"prev_ends_with_comma", []string{"var x = 1,", "var y = 2"}, 1, true},
		{"prev_ends_with_open_paren", []string{"var (", "x = 1"}, 1, true},
		{"prev_is_empty", []string{"", "var x = 1"}, 1, true},
		{"prev_is_code", []string{"func foo() {}", "var x = 1"}, 1, false},
		{"first_line", []string{"var x = 1"}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isContinuation(tt.lines, tt.idx); got != tt.want {
				t.Errorf("isContinuation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractGoVarSymbol(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"var MyVar = 1", "MyVar"},
		{"const MaxSize = 100", "MaxSize"},
		{"var (", "variables"},
		{"x := 1", "variables"},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := extractGoVarSymbol(tt.line); got != tt.want {
				t.Errorf("extractGoVarSymbol(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestChunkGo_MetadataCorrect(t *testing.T) {
	content := `package main

func Hello() {
    fmt.Println("hello")
}`
	chunks := chunkGo(content, "main.go")
	for _, c := range chunks {
		if sym, ok := c.Metadata["symbol"].(string); ok && sym == "Hello" {
			if c.Metadata["language"] != "go" {
				t.Errorf("Hello language = %v, want go", c.Metadata["language"])
			}
			if c.Metadata["file"] != "main.go" {
				t.Errorf("Hello file = %v, want main.go", c.Metadata["file"])
			}
			ls, ok := c.Metadata["line_start"].(int)
			if !ok || ls < 1 {
				t.Errorf("Hello line_start = %v, want >= 1", c.Metadata["line_start"])
			}
		}
	}
}
