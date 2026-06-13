package search

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// LocalJSONLStore persists vectors as one JSON object per line.
type LocalJSONLStore struct {
	path    string
	mu      sync.RWMutex
	loaded  bool
	records []localVectorRecord
}

type localVectorRecord struct {
	ID        string                 `json:"id"`
	Text      string                 `json:"text"`
	Embedding []float32              `json:"embedding"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// NewLocalJSONLStore creates a local JSONL-backed vector store.
func NewLocalJSONLStore(path string) (*LocalJSONLStore, error) {
	if path == "" {
		return nil, fmt.Errorf("VECTOR_STORE_PATH is required for local-jsonl vector store")
	}
	if absPath, err := filepath.Abs(path); err == nil {
		path = absPath
	}
	return &LocalJSONLStore{path: path}, nil
}

// DropCollection removes the JSONL file.
func (s *LocalJSONLStore) DropCollection(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	s.records = nil
	s.loaded = true
	return nil
}

// Close is a no-op for local storage.
func (s *LocalJSONLStore) Close() {}

// HasCollection checks whether the JSONL file exists.
func (s *LocalJSONLStore) HasCollection(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	_, err := os.Stat(s.path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// EnsureCollection creates the parent directory and an empty JSONL file.
func (s *LocalJSONLStore) EnsureCollection(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("创建本地向量目录失败: %w", err)
	}
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("创建本地向量文件失败: %w", err)
	}
	return file.Close()
}

// DeleteByFile removes vectors whose metadata.file matches filePath.
func (s *LocalJSONLStore) DeleteByFile(ctx context.Context, filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	records, err := s.loadRecordsLocked(ctx)
	if err != nil {
		return err
	}

	filtered := records[:0]
	for _, record := range records {
		if f, ok := record.Metadata["file"].(string); ok && f == filePath {
			continue
		}
		filtered = append(filtered, record)
	}
	if err := s.writeRecordsLocked(ctx, filtered); err != nil {
		return err
	}
	s.records = filtered
	s.loaded = true
	return nil
}

// Insert appends vectors to the JSONL file.
func (s *LocalJSONLStore) Insert(ctx context.Context, ids []string, texts []string, vectors [][]float32, metadatas []map[string]interface{}) error {
	if len(ids) != len(texts) || len(ids) != len(vectors) || len(ids) != len(metadatas) {
		return fmt.Errorf("vector insert data length mismatch")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("创建本地向量目录失败: %v", err)
	}

	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开本地向量文件失败: %w", err)
	}
	defer func() { _ = file.Close() }()

	writer := bufio.NewWriter(file)
	encoder := json.NewEncoder(writer)
	for i := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		record := localVectorRecord{
			ID:        ids[i],
			Text:      texts[i],
			Embedding: vectors[i],
			Metadata:  metadatas[i],
		}
		if err := encoder.Encode(record); err != nil {
			return fmt.Errorf("写入本地向量失败: %w", err)
		}
		if s.loaded {
			s.records = append(s.records, record)
		}
	}
	return writer.Flush()
}

// Search performs an in-memory linear cosine search over the JSONL records.
func (s *LocalJSONLStore) Search(ctx context.Context, queryVector []float32, topK int) ([]CodeSearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	if err := s.ensureLoaded(ctx); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]CodeSearchResult, 0, minInt(topK, len(s.records)))
	for _, record := range s.records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		score, ok := cosineSimilarity(queryVector, record.Embedding)
		if !ok {
			continue
		}
		result := CodeSearchResult{
			Text:  record.Text,
			Score: score,
		}
		if f, ok := record.Metadata["file"].(string); ok {
			result.File = f
		}
		results = append(results, result)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// ensureLoaded 确保记录已从磁盘加载（双检锁）
func (s *LocalJSONLStore) ensureLoaded(ctx context.Context) error {
	s.mu.RLock()
	loaded := s.loaded
	s.mu.RUnlock()
	if loaded {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.loadRecordsLocked(ctx)
	return err
}

func (s *LocalJSONLStore) loadRecordsLocked(ctx context.Context) ([]localVectorRecord, error) {
	if s.loaded {
		return s.records, nil
	}

	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.records = nil
			s.loaded = true
			return s.records, nil
		}
		return nil, fmt.Errorf("读取本地向量文件失败: %w", err)
	}
	defer func() { _ = file.Close() }()

	var records []localVectorRecord
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var record localVectorRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, fmt.Errorf("解析本地向量记录失败: %w", err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("扫描本地向量文件失败: %w", err)
	}
	s.records = records
	s.loaded = true
	return s.records, nil
}

// Count 返回当前存储的向量记录数
func (s *LocalJSONLStore) Count(ctx context.Context) (int, error) {
	if err := s.ensureLoaded(ctx); err != nil {
		return 0, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records), nil
}

func (s *LocalJSONLStore) writeRecordsLocked(ctx context.Context, records []localVectorRecord) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("创建本地向量目录失败: %w", err)
	}

	tmpPath := s.path + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("创建本地向量临时文件失败: %w", err)
	}

	writer := bufio.NewWriter(file)
	encoder := json.NewEncoder(writer)
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			_ = file.Close()
			return err
		}
		if err := encoder.Encode(record); err != nil {
			_ = file.Close()
			return fmt.Errorf("写入本地向量临时文件失败: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

func cosineSimilarity(a, b []float32) (float32, bool) {
	if len(a) == 0 || len(a) != len(b) {
		return 0, false
	}

	var dot, normA, normB float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		normA += av * av
		normB += bv * bv
	}
	if normA == 0 || normB == 0 {
		return 0, false
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB))), true
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
