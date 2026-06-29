package tokenstats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Store JSON 文件持久化，原子写入
type Store struct {
	filePath string
}

// NewStore 创建持久化存储
func NewStore(filePath string) *Store {
	return &Store{filePath: filePath}
}

// Load 从文件加载统计快照
// 文件不存在时返回空快照
func (s *Store) Load() (*StatsSnapshot, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return newEmptySnapshot(), nil
		}
		return nil, fmt.Errorf("读取统计文件失败: %w", err)
	}

	var snapshot StatsSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("解析统计文件失败: %w", err)
	}

	if snapshot.ByTool == nil {
		snapshot.ByTool = make(map[string]*ToolStats)
	}

	return &snapshot, nil
}

// Save 保存统计快照到文件（原子写入）
// 调用方应在调用前更新 stats.UpdatedAt
func (s *Store) Save(stats *StatsSnapshot) error {

	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化统计数据失败: %w", err)
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建统计目录失败: %w", err)
	}

	// 原子写入：先写临时文件，再 rename
	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		// Windows 上 rename 可能失败（文件占用），降级为直接写入
		if err2 := os.WriteFile(s.filePath, data, 0644); err2 != nil {
			return fmt.Errorf("写入统计文件失败: %w (rename also failed: %v, tmp preserved at %s)", err2, err, tmpPath)
		}
		// 降级成功，清理 tmp
		_ = os.Remove(tmpPath)
	}

	return nil
}

func newEmptySnapshot() *StatsSnapshot {
	return &StatsSnapshot{
		Version:    "1",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		ByTool:     make(map[string]*ToolStats),
		Daily:      nil,
	}
}
