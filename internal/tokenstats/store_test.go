package tokenstats

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_LoadNotExist(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "nonexistent.json"))
	snapshot, err := store.Load()
	if err != nil {
		t.Fatalf("Load nonexistent file should not error: %v", err)
	}
	if snapshot == nil {
		t.Fatal("Load should return empty snapshot, got nil")
	}
	if snapshot.Version != "1" {
		t.Errorf("Version = %q, want \"1\"", snapshot.Version)
	}
	if snapshot.ByTool == nil {
		t.Error("ByTool should be initialized as empty map")
	}
	if len(snapshot.ByTool) != 0 {
		t.Errorf("ByTool should be empty, got %d entries", len(snapshot.ByTool))
	}
}

func TestStore_SaveLoadConsistency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.json")
	store := NewStore(path)

	original := &StatsSnapshot{
		Version:           "1",
		CreatedAt:         time.Now(),
		TotalCalls:        42,
		TotalOutputTokens: 10000,
		TotalSavedTokens:  5000,
		ByTool: map[string]*ToolStats{
			"code_search": {
				CallCount:         20,
				TotalOutputTokens: 5000,
				TotalSavedTokens:  3000,
				TotalDurationMs:   1200,
			},
		},
		Daily: []DailyStats{
			{Date: "2026-06-01", CallCount: 20, OutputTokens: 5000, SavedTokens: 3000},
			{Date: "2026-06-02", CallCount: 22, OutputTokens: 5000, SavedTokens: 2000},
		},
	}

	if err := store.Save(original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.TotalCalls != 42 {
		t.Errorf("TotalCalls = %d, want 42", loaded.TotalCalls)
	}
	if loaded.TotalOutputTokens != 10000 {
		t.Errorf("TotalOutputTokens = %d, want 10000", loaded.TotalOutputTokens)
	}
	if loaded.TotalSavedTokens != 5000 {
		t.Errorf("TotalSavedTokens = %d, want 5000", loaded.TotalSavedTokens)
	}
	if len(loaded.ByTool) != 1 {
		t.Errorf("ByTool len = %d, want 1", len(loaded.ByTool))
	}
	if cs, ok := loaded.ByTool["code_search"]; !ok {
		t.Error("code_search not found in ByTool")
	} else if cs.CallCount != 20 {
		t.Errorf("code_search CallCount = %d, want 20", cs.CallCount)
	}
	if len(loaded.Daily) != 2 {
		t.Errorf("Daily len = %d, want 2", len(loaded.Daily))
	}
}

func TestStore_SaveCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subdir", "stats.json")
	store := NewStore(path)

	snapshot := newEmptySnapshot()
	if err := store.Save(snapshot); err != nil {
		t.Fatalf("Save should create parent dirs: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("File should exist after Save: %v", err)
	}
}
