package indexer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/handy-h/code-context-mcp/internal/types"
)

func TestComputeMtimeFingerprint(t *testing.T) {
	mtimes := map[string]string{
		"a.go": "2024-01-01T00:00:00Z",
		"b.go": "2024-01-02T00:00:00Z",
	}
	fp := computeMtimeFingerprint(mtimes)
	if len(fp) != 16 {
		t.Errorf("fingerprint length = %d, want 16", len(fp))
	}

	// Same input should produce same output
	fp2 := computeMtimeFingerprint(mtimes)
	if fp != fp2 {
		t.Errorf("fingerprint not deterministic: %q vs %q", fp, fp2)
	}
}

func TestComputeMtimeFingerprint_Empty(t *testing.T) {
	fp := computeMtimeFingerprint(map[string]string{})
	if len(fp) != 16 {
		t.Errorf("empty fingerprint length = %d, want 16", len(fp))
	}
}

func TestComputeMtimeFingerprint_DifferentInputs(t *testing.T) {
	fp1 := computeMtimeFingerprint(map[string]string{"a.go": "2024-01-01T00:00:00Z"})
	fp2 := computeMtimeFingerprint(map[string]string{"a.go": "2024-01-02T00:00:00Z"})
	if fp1 == fp2 {
		t.Error("different mtimes should produce different fingerprints")
	}
}

func TestNewIndexStateStore_DefaultPath(t *testing.T) {
	store := NewIndexStateStore("/project", "")
	// 默认路径不应基于 projectPath（改为 exe_dir 基准）
	if strings.Contains(store.filePath, "/project") {
		t.Errorf("default path should not be based on projectPath, got %q", store.filePath)
	}
	// 应以 index-state.json 结尾
	if !strings.HasSuffix(store.filePath, "index-state.json") {
		t.Errorf("default path should end with index-state.json, got %q", store.filePath)
	}
}

func TestNewIndexStateStore_CustomPath(t *testing.T) {
	store := NewIndexStateStore("/project", "/custom/state.json")
	if store.filePath != "/custom/state.json" {
		t.Errorf("custom path = %q", store.filePath)
	}
}

func TestIndexStateStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewIndexStateStore(dir, filepath.Join(dir, "state.json"))

	state := &types.IndexState{
		Fingerprint: "abc123",
		TotalFiles:  10,
		TotalChunks: 50,
		ProjectPath: dir,
		FileMtimes:  map[string]string{"a.go": "2024-01-01T00:00:00Z"},
	}

	if err := store.Save(state); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.Fingerprint != "abc123" {
		t.Errorf("Fingerprint = %q, want abc123", loaded.Fingerprint)
	}
	if loaded.TotalFiles != 10 {
		t.Errorf("TotalFiles = %d, want 10", loaded.TotalFiles)
	}
	if loaded.TotalChunks != 50 {
		t.Errorf("TotalChunks = %d, want 50", loaded.TotalChunks)
	}
}

func TestIndexStateStore_Load_NoFile(t *testing.T) {
	dir := t.TempDir()
	store := NewIndexStateStore(dir, filepath.Join(dir, "nonexistent.json"))

	_, err := store.Load()
	if !os.IsNotExist(err) {
		t.Errorf("Load(no file) error = %v, want os.ErrNotExist", err)
	}
}

func TestIndexStateStore_Load_Corrupted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	store := NewIndexStateStore(dir, path)
	_, err := store.Load()
	if err == nil {
		t.Error("Load(corrupted) should return error")
	}
}

func TestIndexStateStore_Save_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "state.json")
	store := NewIndexStateStore(dir, path)

	state := &types.IndexState{Fingerprint: "test"}
	if err := store.Save(state); err != nil {
		t.Fatalf("Save should create dir, got error: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Save should create the file")
	}
}

func TestGetGitCommitHash_InGitRepo(t *testing.T) {
	hash, err := getGitCommitHash(".")
	if err != nil {
		t.Skipf("not in a git repo: %v", err)
	}
	if len(hash) == 0 {
		t.Error("git commit hash should not be empty")
	}
}

func TestGetGitCommitHash_NoGitDir(t *testing.T) {
	dir := t.TempDir()
	_, err := getGitCommitHash(dir)
	if err == nil {
		t.Error("getGitCommitHash(non-git dir) should return error")
	}
}

func TestScanFileMtimes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("text"), 0644); err != nil {
		t.Fatal(err)
	}

	mtimes, err := scanFileMtimes(dir, []string{".go"})
	if err != nil {
		t.Fatalf("scanFileMtimes error: %v", err)
	}
	if len(mtimes) != 2 {
		t.Errorf("scanFileMtimes found %d files, want 2", len(mtimes))
	}
	if _, ok := mtimes["a.go"]; !ok {
		t.Error("a.go not found in mtimes")
	}
	if _, ok := mtimes["c.txt"]; ok {
		t.Error("c.txt should not be included (wrong extension)")
	}
}

func TestShouldSkipDir(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"node_modules", true},
		{".git", true},
		{"dist", true},
		{".venv", true},
		{"vendor", true},
		{"__pycache__", true},
		{"src", false},
		{"internal", false},
		{"pkg", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSkipDir(tt.name); got != tt.want {
				t.Errorf("shouldSkipDir(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestSaveFromStats(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	store := NewIndexStateStore(dir, filepath.Join(dir, "state.json"))
	stats := &IndexStats{TotalFiles: 1, TotalChunks: 5}

	if err := store.SaveFromStats(dir, stats, []string{".go"}); err != nil {
		t.Fatalf("SaveFromStats error: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", loaded.TotalFiles)
	}
	if loaded.TotalChunks != 5 {
		t.Errorf("TotalChunks = %d, want 5", loaded.TotalChunks)
	}
}
