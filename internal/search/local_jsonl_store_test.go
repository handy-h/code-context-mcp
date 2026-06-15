package search

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestCosineSimilarity_Identical(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{1, 2, 3}
	sim, ok := cosineSimilarity(a, b)
	if !ok {
		t.Fatal("cosineSimilarity returned false for identical vectors")
	}
	if math.Abs(float64(sim)-1.0) > 0.001 {
		t.Errorf("cosineSimilarity(identical) = %f, want 1.0", sim)
	}
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	sim, ok := cosineSimilarity(a, b)
	if !ok {
		t.Fatal("cosineSimilarity returned false")
	}
	if math.Abs(float64(sim)) > 0.001 {
		t.Errorf("cosineSimilarity(orthogonal) = %f, want 0.0", sim)
	}
}

func TestCosineSimilarity_Opposite(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{-1, -2, -3}
	sim, ok := cosineSimilarity(a, b)
	if !ok {
		t.Fatal("cosineSimilarity returned false")
	}
	if math.Abs(float64(sim)-(-1.0)) > 0.001 {
		t.Errorf("cosineSimilarity(opposite) = %f, want -1.0", sim)
	}
}

func TestCosineSimilarity_EmptyVector(t *testing.T) {
	_, ok := cosineSimilarity([]float32{}, []float32{})
	if ok {
		t.Error("cosineSimilarity(empty) should return false")
	}
}

func TestCosineSimilarity_DifferentLength(t *testing.T) {
	_, ok := cosineSimilarity([]float32{1, 2}, []float32{1, 2, 3})
	if ok {
		t.Error("cosineSimilarity(different length) should return false")
	}
}

func TestMinInt(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{1, 2, 1},
		{5, 3, 3},
		{4, 4, 4},
		{-1, 0, -1},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got := min(tt.a, tt.b); got != tt.want {
				t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestNewLocalJSONLStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	store, err := NewLocalJSONLStore(path)
	if err != nil {
		t.Fatalf("NewLocalJSONLStore error: %v", err)
	}
	if store == nil {
		t.Fatal("NewLocalJSONLStore returned nil")
	}
}

func TestNewLocalJSONLStore_EmptyPath(t *testing.T) {
	_, err := NewLocalJSONLStore("")
	if err == nil {
		t.Error("NewLocalJSONLStore(empty path) should return error")
	}
}

func TestLocalJSONLStore_InsertAndSearch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	store, err := NewLocalJSONLStore(path)
	if err != nil {
		t.Fatalf("NewLocalJSONLStore error: %v", err)
	}

	ctx := context.Background()
	if err := store.EnsureCollection(ctx); err != nil {
		t.Fatalf("EnsureCollection error: %v", err)
	}

	ids := []string{"doc_1", "doc_2"}
	texts := []string{"func Hello() {}", "func World() {}"}
	vectors := [][]float32{{1, 0, 0}, {0, 1, 0}}
	metadatas := []map[string]interface{}{
		{"file": "a.go"},
		{"file": "b.go"},
	}

	if err := store.Insert(ctx, ids, texts, vectors, metadatas); err != nil {
		t.Fatalf("Insert error: %v", err)
	}

	results, err := store.Search(ctx, []float32{1, 0, 0}, 2)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search returned empty results")
	}
	if results[0].File != "a.go" {
		t.Errorf("top result file = %q, want a.go", results[0].File)
	}
}

func TestLocalJSONLStore_Search_TopK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	store, _ := NewLocalJSONLStore(path)
	ctx := context.Background()
	if err := store.EnsureCollection(ctx); err != nil {
		t.Fatal(err)
	}

	ids := []string{"doc_1", "doc_2", "doc_3"}
	texts := []string{"a", "b", "c"}
	vectors := [][]float32{{1, 0, 0}, {0.9, 0.1, 0}, {0.8, 0.2, 0}}
	metadatas := []map[string]interface{}{{}, {}, {}}
	if err := store.Insert(ctx, ids, texts, vectors, metadatas); err != nil {
		t.Fatal(err)
	}

	results, err := store.Search(ctx, []float32{1, 0, 0}, 2)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) > 2 {
		t.Errorf("Search topK=2 returned %d results", len(results))
	}
}

func TestLocalJSONLStore_DeleteByFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	store, _ := NewLocalJSONLStore(path)
	ctx := context.Background()
	if err := store.EnsureCollection(ctx); err != nil {
		t.Fatal(err)
	}

	ids := []string{"doc_1", "doc_2"}
	texts := []string{"a", "b"}
	vectors := [][]float32{{1, 0, 0}, {0, 1, 0}}
	metadatas := []map[string]interface{}{
		{"file": "a.go"},
		{"file": "b.go"},
	}
	if err := store.Insert(ctx, ids, texts, vectors, metadatas); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteByFile(ctx, "a.go"); err != nil {
		t.Fatalf("DeleteByFile error: %v", err)
	}

	results, _ := store.Search(ctx, []float32{1, 0, 0}, 10)
	for _, r := range results {
		if r.File == "a.go" {
			t.Error("results from a.go should be deleted")
		}
	}
}

func TestLocalJSONLStore_DropCollection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	store, _ := NewLocalJSONLStore(path)
	ctx := context.Background()
	if err := store.EnsureCollection(ctx); err != nil {
		t.Fatal(err)
	}

	if err := store.DropCollection(ctx); err != nil {
		t.Fatalf("DropCollection error: %v", err)
	}

	has, _ := store.HasCollection(ctx)
	if has {
		t.Error("HasCollection should return false after DropCollection")
	}
}

func TestLocalJSONLStore_HasCollection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	store, _ := NewLocalJSONLStore(path)
	ctx := context.Background()

	has, _ := store.HasCollection(ctx)
	if has {
		t.Error("HasCollection should return false before EnsureCollection")
	}

	if err := store.EnsureCollection(ctx); err != nil {
		t.Fatal(err)
	}
	has, _ = store.HasCollection(ctx)
	if !has {
		t.Error("HasCollection should return true after EnsureCollection")
	}
}

func TestLocalJSONLStore_EnsureCollection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "test.jsonl")
	store, _ := NewLocalJSONLStore(path)
	ctx := context.Background()

	if err := store.EnsureCollection(ctx); err != nil {
		t.Fatalf("EnsureCollection error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("EnsureCollection should create the file")
	}
}

func TestLocalJSONLStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	// Write data
	store1, _ := NewLocalJSONLStore(path)
	ctx := context.Background()
	if err := store1.EnsureCollection(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store1.Insert(ctx, []string{"doc_1"}, []string{"hello"}, [][]float32{{1, 2, 3}}, []map[string]interface{}{{"file": "a.go"}}); err != nil {
		t.Fatal(err)
	}

	// Reload and verify
	store2, _ := NewLocalJSONLStore(path)
	results, err := store2.Search(ctx, []float32{1, 2, 3}, 1)
	if err != nil {
		t.Fatalf("Search after reload error: %v", err)
	}
	if len(results) == 0 {
		t.Error("data should persist after reload")
	}
}

func TestLocalJSONLStore_Close(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	store, _ := NewLocalJSONLStore(path)
	store.Close() // should not panic
}

func TestLocalJSONLStore_ContextCanceled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	store, _ := NewLocalJSONLStore(path)

	ctx, cancel := context.WithCancel(context.Background())
	if err := store.EnsureCollection(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Insert(ctx, []string{"doc_1"}, []string{"hello"}, [][]float32{{1, 2, 3}}, []map[string]interface{}{{}}); err != nil {
		t.Fatal(err)
	}

	cancel()
	_, err := store.Search(ctx, []float32{1, 2, 3}, 1)
	if err == nil {
		t.Error("Search with canceled context should return error")
	}
}

func TestLocalJSONLStore_Insert_MismatchedLengths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	store, _ := NewLocalJSONLStore(path)
	ctx := context.Background()
	if err := store.EnsureCollection(ctx); err != nil {
		t.Fatal(err)
	}

	err := store.Insert(ctx, []string{"doc_1"}, []string{"a", "b"}, [][]float32{{1}}, []map[string]interface{}{{}})
	if err == nil {
		t.Error("Insert with mismatched lengths should return error")
	}
}
