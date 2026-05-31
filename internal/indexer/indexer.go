package indexer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/handy-h/code-context-mcp/internal/config"
	"github.com/handy-h/code-context-mcp/internal/embedding"
	"github.com/handy-h/code-context-mcp/internal/search"
	"github.com/handy-h/code-context-mcp/pkg/structure"
)

// IndexStats 索引统计信息
type IndexStats struct {
	TotalFiles  int `json:"total_files"`
	TotalChunks int `json:"total_chunks"`
}

// document 扫描到的文件文档
type document struct {
	FilePath string
	Content  string
}

// shouldSkipDir 判断是否应跳过该目录（不参与索引）
func shouldSkipDir(name string) bool {
	switch name {
	case "node_modules", ".git", "dist", ".venv", "vendor", "__pycache__":
		return true
	}
	return false
}

// ScanFiles 扫描指定目录下的目标文件
func ScanFiles(root string, extensions []string) ([]document, error) {
	var docs []document
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		for _, target := range extensions {
			if ext == target {
				content, err := os.ReadFile(path)
				if err != nil {
					slog.Warn("读取文件失败", "path", path, "err", err)
					break
				}
				// 使用相对路径
				relPath, err := filepath.Rel(root, path)
				if err != nil {
					relPath = path
				}
				docs = append(docs, document{FilePath: relPath, Content: string(content)})
				break
			}
		}
		return nil
	})
	return docs, err
}

const batchSize = 500

// flushBatch 将累积的向量批量写入存储
func flushBatch(ctx context.Context, vdb search.VectorStore, ids, texts []string, vectors [][]float32, metadatas []map[string]interface{}) error {
	if len(vectors) == 0 {
		return nil
	}
	slog.Info("批量插入向量", "count", len(vectors))
	if err := vdb.Insert(ctx, ids, texts, vectors, metadatas); err != nil {
		return fmt.Errorf("插入失败: %w", err)
	}
	return nil
}

// BuildIndex 构建项目代码索引：扫描 → 切分 → 向量化 → 插入
func BuildIndex(ctx context.Context, projectPath string, cfg config.Config, vdb search.VectorStore, invIndex *search.InvertedIndex) (*IndexStats, error) {
	// 1. 扫描文件
	docs, err := ScanFiles(projectPath, cfg.ScanExtensions)
	if err != nil {
		return nil, fmt.Errorf("扫描文件失败: %w", err)
	}
	slog.Info("文件扫描完成", "count", len(docs))

	// 2. 丢弃旧集合，重建干净的索引
	if err := vdb.DropCollection(ctx); err != nil {
		slog.Warn("丢弃旧集合失败（首次运行可忽略）", "err", err)
	}

	// 3. 确保集合存在
	if err := vdb.EnsureCollection(ctx); err != nil {
		return nil, fmt.Errorf("确保集合失败: %w", err)
	}

	// 4. 切分 + 向量化（分批写入，避免大项目 OOM）
	var allVectors [][]float32
	var allTexts []string
	var allIDs []string
	var allMetadatas []map[string]interface{}

	chunkIdx := 0
	totalChunks := 0
	for _, doc := range docs {
		lang := structure.DetectLanguage(doc.FilePath)
		chunks := structure.SplitByStructure(doc.Content, lang, doc.FilePath, cfg.MaxChunkSize)

		// 构建倒排索引
		if invIndex != nil {
			invIndex.BuildFromChunks(chunks, doc.FilePath)
		}

		for _, chunk := range chunks {
			vector, err := embedding.GetEmbedding(cfg, chunk.Content)
			if err != nil {
				slog.Warn("向量化失败", "file", doc.FilePath, "err", err)
				continue
			}

			chunkIdx++
			totalChunks++
			allIDs = append(allIDs, fmt.Sprintf("doc_%d", chunkIdx))
			allTexts = append(allTexts, chunk.Content)
			allVectors = append(allVectors, vector)
			allMetadatas = append(allMetadatas, chunk.Metadata)

			if len(allVectors) >= batchSize {
				if err := flushBatch(ctx, vdb, allIDs, allTexts, allVectors, allMetadatas); err != nil {
					return nil, err
				}
				allIDs = allIDs[:0]
				allTexts = allTexts[:0]
				allVectors = allVectors[:0]
				allMetadatas = allMetadatas[:0]
			}
		}
	}

	// 5. 写入剩余批次
	if err := flushBatch(ctx, vdb, allIDs, allTexts, allVectors, allMetadatas); err != nil {
		return nil, err
	}

	return &IndexStats{
		TotalFiles:  len(docs),
		TotalChunks: totalChunks,
	}, nil
}
