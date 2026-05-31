package search

import (
	"context"
	"testing"

	"github.com/handy-h/code-context-mcp/internal/config"
)

func TestNewVectorDB_LocalJSONL(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		VectorStore:     config.VectorStoreLocalJSONL,
		VectorStorePath: dir + "/test.jsonl",
	}
	ctx := context.Background()
	vdb, err := NewVectorDB(ctx, cfg)
	if err != nil {
		t.Fatalf("NewVectorDB(local) error: %v", err)
	}
	defer vdb.Close()

	if _, ok := vdb.(*LocalJSONLStore); !ok {
		t.Errorf("NewVectorDB(local) returned %T, want *LocalJSONLStore", vdb)
	}
}

func TestNewVectorDB_Unsupported(t *testing.T) {
	cfg := config.Config{
		VectorStore: "unsupported",
	}
	ctx := context.Background()
	_, err := NewVectorDB(ctx, cfg)
	if err == nil {
		t.Error("NewVectorDB(unsupported) should return error")
	}
}
