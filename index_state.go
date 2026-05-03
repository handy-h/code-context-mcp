package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// IndexStateStore 索引状态持久化存储
type IndexStateStore struct {
	filePath string
}

// NewIndexStateStore 创建状态存储
func NewIndexStateStore(projectPath string, statePath string) *IndexStateStore {
	if statePath != "" {
		return &IndexStateStore{filePath: statePath}
	}
	return &IndexStateStore{
		filePath: filepath.Join(projectPath, ".code-context-index-state.json"),
	}
}

// Load 从文件加载索引状态
func (s *IndexStateStore) Load() (*IndexState, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("读取索引状态文件失败: %v", err)
	}

	var state IndexState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("解析索引状态文件失败: %v", err)
	}

	return &state, nil
}

// Save 保存索引状态到文件
func (s *IndexStateStore) Save(state *IndexState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化索引状态失败: %v", err)
	}

	// 确保目录存在
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建索引状态目录失败: %v", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("写入索引状态文件失败: %v", err)
	}

	return nil
}

// GetCurrentFingerprint 计算当前项目的索引指纹
// 优先尝试 git commit hash，失败则降级为文件 mtime 摘要
func (s *IndexStateStore) GetCurrentFingerprint(projectPath string, extensions []string) (string, map[string]string, error) {
	// 优先尝试 git commit hash
	if hash, err := getGitCommitHash(projectPath); err == nil && hash != "" {
		mtimes, _ := scanFileMtimes(projectPath, extensions)
		return hash, mtimes, nil
	}

	// 降级为文件 mtime 摘要
	mtimes, err := scanFileMtimes(projectPath, extensions)
	if err != nil {
		return "", nil, fmt.Errorf("计算文件 mtime 摘要失败: %v", err)
	}

	fingerprint := computeMtimeFingerprint(mtimes)
	return fingerprint, mtimes, nil
}

// getGitCommitHash 获取 git commit hash
func getGitCommitHash(projectPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = projectPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// scanFileMtimes 扫描文件修改时间
func scanFileMtimes(projectPath string, extensions []string) (map[string]string, error) {
	mtimes := make(map[string]string)
	extMap := make(map[string]bool, len(extensions))
	for _, ext := range extensions {
		extMap[ext] = true
	}

	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == ".git" || name == "dist" ||
				name == ".venv" || name == "vendor" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if extMap[ext] {
			relPath, err := filepath.Rel(projectPath, path)
			if err != nil {
				relPath = path
			}
			mtimes[relPath] = info.ModTime().Format(time.RFC3339Nano)
		}
		return nil
	})

	return mtimes, err
}

// computeMtimeFingerprint 计算文件 mtime 摘要
func computeMtimeFingerprint(mtimes map[string]string) string {
	h := sha256.New()
	for path, mtime := range mtimes {
		h.Write([]byte(path + ":" + mtime + "\n"))
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}
