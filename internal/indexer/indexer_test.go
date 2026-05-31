package indexer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanFiles_Empty(t *testing.T) {
	dir := t.TempDir()
	docs, err := ScanFiles(dir, []string{".go"})
	if err != nil {
		t.Fatalf("ScanFiles error: %v", err)
	}
	if len(docs) != 0 {
		t.Errorf("ScanFiles(empty dir) = %d files, want 0", len(docs))
	}
}

func TestScanFiles_Extensions(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log()"), 0644)
	os.WriteFile(filepath.Join(dir, "style.css"), []byte("body{}"), 0644)

	docs, err := ScanFiles(dir, []string{".go", ".js"})
	if err != nil {
		t.Fatalf("ScanFiles error: %v", err)
	}
	if len(docs) != 2 {
		t.Errorf("ScanFiles = %d files, want 2", len(docs))
	}
}

func TestScanFiles_SkipDirs(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)

	nodeModules := filepath.Join(dir, "node_modules")
	os.MkdirAll(nodeModules, 0755)
	os.WriteFile(filepath.Join(nodeModules, "lib.go"), []byte("package lib"), 0644)

	gitDir := filepath.Join(dir, ".git")
	os.MkdirAll(gitDir, 0755)
	os.WriteFile(filepath.Join(gitDir, "hook.go"), []byte("package hook"), 0644)

	docs, err := ScanFiles(dir, []string{".go"})
	if err != nil {
		t.Fatalf("ScanFiles error: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("ScanFiles = %d files, want 1 (should skip node_modules/.git)", len(docs))
	}
}

func TestScanFiles_RelativePaths(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "pkg")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "util.go"), []byte("package util"), 0644)

	docs, err := ScanFiles(dir, []string{".go"})
	if err != nil {
		t.Fatalf("ScanFiles error: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("ScanFiles returned 0 files")
	}
	if filepath.IsAbs(docs[0].FilePath) {
		t.Errorf("FilePath should be relative, got %q", docs[0].FilePath)
	}
}

func TestScanFiles_NestedDirs(t *testing.T) {
	dir := t.TempDir()
	deep := filepath.Join(dir, "a", "b", "c")
	os.MkdirAll(deep, 0755)
	os.WriteFile(filepath.Join(deep, "deep.go"), []byte("package deep"), 0644)

	docs, err := ScanFiles(dir, []string{".go"})
	if err != nil {
		t.Fatalf("ScanFiles error: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("ScanFiles = %d files, want 1", len(docs))
	}
}

func TestScanFiles_ContentRead(t *testing.T) {
	dir := t.TempDir()
	content := "package main\n\nfunc main() {}"
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0644)

	docs, err := ScanFiles(dir, []string{".go"})
	if err != nil {
		t.Fatalf("ScanFiles error: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("ScanFiles returned 0 files")
	}
	if docs[0].Content != content {
		t.Errorf("Content = %q, want %q", docs[0].Content, content)
	}
}
