package tokenstats

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestTracker_Disabled(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "stats.json"))
	tracker := NewTracker(store, BaselineConfig{CodeSearchFileTokens: 2000}, 4.0, false, 90)

	err := tracker.Record(ToolCallRecord{
		ToolName:   "code_search",
		Args:       map[string]interface{}{"top_k": 5.0},
		OutputText: "some output",
		DurationMs: 100,
		Timestamp:  time.Now(),
	})
	if err != nil {
		t.Fatalf("Record should not error when disabled: %v", err)
	}

	stats := tracker.GetStats()
	if stats.TotalCalls != 0 {
		t.Errorf("TotalCalls = %d, want 0 when disabled", stats.TotalCalls)
	}
}

func TestTracker_SkipTracking(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "stats.json"))
	tracker := NewTracker(store, BaselineConfig{CodeSearchFileTokens: 2000}, 4.0, true, 90)

	for _, tool := range []string{"index_project", "token_stats"} {
		err := tracker.Record(ToolCallRecord{
			ToolName:   tool,
			OutputText: "some output",
			Timestamp:  time.Now(),
		})
		if err != nil {
			t.Fatalf("Record should not error: %v", err)
		}
	}

	stats := tracker.GetStats()
	if stats.TotalCalls != 0 {
		t.Errorf("TotalCalls = %d, want 0 for skipped tools", stats.TotalCalls)
	}
}

func TestTracker_ConcurrentRecord(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "stats.json"))
	tracker := NewTracker(store, BaselineConfig{
		CodeSearchFileTokens:   2000,
		FileContextBaseline:    3000,
		SymbolSearchBaseline:   8000,
		ImpactAnalysisBaseline: 12000,
	}, 4.0, true, 90)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = tracker.Record(ToolCallRecord{
				ToolName:   "code_search",
				Args:       map[string]interface{}{"top_k": 5.0},
				OutputText: "some output text here",
				DurationMs: int64(n),
				Timestamp:  time.Now(),
			})
		}(i)
	}
	wg.Wait()

	stats := tracker.GetStats()
	if stats.TotalCalls != 100 {
		t.Errorf("TotalCalls = %d, want 100", stats.TotalCalls)
	}

	cs, ok := stats.ByTool["code_search"]
	if !ok {
		t.Fatal("code_search not in ByTool")
	}
	if cs.CallCount != 100 {
		t.Errorf("code_search CallCount = %d, want 100", cs.CallCount)
	}
}

func TestTracker_DailyAggregation(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "stats.json"))
	tracker := NewTracker(store, BaselineConfig{
		CodeSearchFileTokens: 2000,
	}, 4.0, true, 90)

	// 记录两条同一天的调用
	now := time.Now()
	_ = tracker.Record(ToolCallRecord{
		ToolName:   "code_search",
		Args:       map[string]interface{}{"top_k": 1.0},
		OutputText: "short",
		Timestamp:  now,
	})
	_ = tracker.Record(ToolCallRecord{
		ToolName:   "code_search",
		Args:       map[string]interface{}{"top_k": 1.0},
		OutputText: "another short",
		Timestamp:  now.Add(1 * time.Hour),
	})

	stats := tracker.GetStats()
	today := now.Format("2006-01-02")
	found := false
	for _, d := range stats.Daily {
		if d.Date == today {
			if d.CallCount != 2 {
				t.Errorf("Daily CallCount = %d, want 2", d.CallCount)
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Daily entry for %s not found", today)
	}
}

func TestTracker_RetentionPruning(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "stats.json"))
	tracker := NewTracker(store, BaselineConfig{
		CodeSearchFileTokens: 2000,
	}, 4.0, true, 7) // 只保留7天

	now := time.Now()

	// 记录一条旧的（30天前）和一条新的
	_ = tracker.Record(ToolCallRecord{
		ToolName:   "code_search",
		Args:       map[string]interface{}{"top_k": 1.0},
		OutputText: "old",
		Timestamp:  now.AddDate(0, 0, -30),
	})

	// 清空 daily，手动添加旧数据来测试裁剪
	tracker.mu.Lock()
	tracker.snapshot.Daily = []DailyStats{
		{Date: now.AddDate(0, 0, -30).Format("2006-01-02"), CallCount: 1},
		{Date: now.AddDate(0, 0, -10).Format("2006-01-02"), CallCount: 1},
		{Date: now.AddDate(0, 0, -3).Format("2006-01-02"), CallCount: 1},
		{Date: now.Format("2006-01-02"), CallCount: 1},
	}
	tracker.mu.Unlock()

	// 记录新数据触发裁剪
	_ = tracker.Record(ToolCallRecord{
		ToolName:   "code_search",
		Args:       map[string]interface{}{"top_k": 1.0},
		OutputText: "new",
		Timestamp:  now,
	})

	stats := tracker.GetStats()
	cutoff := now.AddDate(0, 0, -7).Format("2006-01-02")
	for _, d := range stats.Daily {
		if d.Date < cutoff {
			t.Errorf("Daily entry %q should have been pruned (cutoff=%q)", d.Date, cutoff)
		}
	}
}

func TestTracker_Flush(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "stats.json"))
	tracker := NewTracker(store, BaselineConfig{
		CodeSearchFileTokens: 2000,
	}, 4.0, true, 90)

	_ = tracker.Record(ToolCallRecord{
		ToolName:   "code_search",
		Args:       map[string]interface{}{"top_k": 1.0},
		OutputText: "test output",
		Timestamp:  time.Now(),
	})

	// Flush 应该强制落盘
	if err := tracker.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// 重新加载验证
	store2 := NewStore(store.filePath)
	loaded, err := store2.Load()
	if err != nil {
		t.Fatalf("Load after Flush failed: %v", err)
	}
	if loaded.TotalCalls != 1 {
		t.Errorf("After Flush: TotalCalls = %d, want 1", loaded.TotalCalls)
	}
}
