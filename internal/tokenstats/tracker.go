package tokenstats

import (
	"log/slog"
	"sync"
	"time"
)

// Tracker 并发安全的统计记录器
type Tracker struct {
	mu            sync.Mutex
	store         *Store
	baseline      BaselineConfig
	charsPerToken float64
	enabled       bool
	snapshot      *StatsSnapshot
	retentionDays int
	dirty         bool
	lastSaveTime  time.Time
}

// NewTracker 创建统计追踪器
func NewTracker(store *Store, baseline BaselineConfig, charsPerToken float64, enabled bool, retentionDays int) *Tracker {
	snapshot, err := store.Load()
	if err != nil {
		// 文件损坏等错误以 warn 记录，但不阻断：用户可能手动修复
		// 此时使用空快照启动，历史统计丢失但服务正常运行
		slog.Warn("加载统计文件失败，使用空快照", "err", err)
		snapshot = newEmptySnapshot()
	}
	return &Tracker{
		store:         store,
		baseline:      baseline,
		charsPerToken: charsPerToken,
		enabled:       enabled,
		snapshot:      snapshot,
		retentionDays: retentionDays,
		lastSaveTime:  time.Now(),
	}
}

// skipTracking 过滤不参与统计的工具
var skipTracking = map[string]bool{
	"index_project": true,
	"token_stats":   true,
}

// Record 记录一次工具调用
func (t *Tracker) Record(rec ToolCallRecord) error {
	if !t.enabled {
		return nil
	}
	if skipTracking[rec.ToolName] {
		return nil
	}

	outputTokens := EstimateTokens(rec.OutputText, t.charsPerToken)
	savedTokens, wastedTokens := t.baseline.CalculateMetrics(rec.ToolName, rec.Args, outputTokens, rec.ResultQuality)

	t.mu.Lock()
	defer t.mu.Unlock()

	// 更新总量
	t.snapshot.TotalCalls++
	t.snapshot.TotalOutputTokens += int64(outputTokens)
	t.snapshot.TotalSavedTokens += int64(savedTokens)
	t.snapshot.TotalWastedTokens += int64(wastedTokens)

	// 更新 by_tool
	tool, ok := t.snapshot.ByTool[rec.ToolName]
	if !ok {
		tool = &ToolStats{}
		t.snapshot.ByTool[rec.ToolName] = tool
	}
	tool.CallCount++
	tool.TotalOutputTokens += int64(outputTokens)
	tool.TotalSavedTokens += int64(savedTokens)
	tool.TotalWastedTokens += int64(wastedTokens)
	tool.TotalDurationMs += rec.DurationMs

	// 更新 daily
	dateStr := rec.Timestamp.Format("2006-01-02")
	t.updateDaily(dateStr, int64(outputTokens), int64(savedTokens), int64(wastedTokens))

	// 裁剪过期 daily
	t.pruneDaily(rec.Timestamp)

	// 条件落盘：距上次落盘超过 30 秒
	t.dirty = true
	if time.Since(t.lastSaveTime) >= 30*time.Second {
		t.snapshot.UpdatedAt = time.Now()
		if err := t.store.Save(t.snapshot); err != nil {
			return err
		}
		t.dirty = false
		t.lastSaveTime = time.Now()
	}

	return nil
}

// GetStats 返回统计快照副本
func (t *Tracker) GetStats() StatsSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 浅拷贝足够：调用方不修改 map 内部
	copied := *t.snapshot
	copied.ByTool = make(map[string]*ToolStats, len(t.snapshot.ByTool))
	for k, v := range t.snapshot.ByTool {
		tv := *v
		copied.ByTool[k] = &tv
	}
	copied.Daily = append([]DailyStats(nil), t.snapshot.Daily...)
	return copied
}

// Flush 强制落盘（优雅退出时调用）
func (t *Tracker) Flush() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.dirty {
		return nil
	}

	t.snapshot.UpdatedAt = time.Now()
	if err := t.store.Save(t.snapshot); err != nil {
		return err
	}
	t.dirty = false
	t.lastSaveTime = time.Now()
	return nil
}

func (t *Tracker) updateDaily(dateStr string, outputTokens, savedTokens, wastedTokens int64) {
	// 查找现有条目
	for i := len(t.snapshot.Daily) - 1; i >= 0; i-- {
		if t.snapshot.Daily[i].Date == dateStr {
			t.snapshot.Daily[i].CallCount++
			t.snapshot.Daily[i].OutputTokens += outputTokens
			t.snapshot.Daily[i].SavedTokens += savedTokens
			t.snapshot.Daily[i].WastedTokens += wastedTokens
			return
		}
	}

	// 新增条目
	t.snapshot.Daily = append(t.snapshot.Daily, DailyStats{
		Date:         dateStr,
		CallCount:    1,
		OutputTokens: outputTokens,
		SavedTokens:  savedTokens,
		WastedTokens: wastedTokens,
	})
}

func (t *Tracker) pruneDaily(now time.Time) {
	if t.retentionDays <= 0 {
		return
	}

	cutoff := now.AddDate(0, 0, -t.retentionDays).Format("2006-01-02")
	var pruned []DailyStats
	for _, d := range t.snapshot.Daily {
		if d.Date >= cutoff {
			pruned = append(pruned, d)
		}
	}
	t.snapshot.Daily = pruned
}
