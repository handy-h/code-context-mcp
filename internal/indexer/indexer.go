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
	"github.com/handy-h/code-context-mcp/internal/types"
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
	case "node_modules", ".git", "dist", ".venv", "vendor", "__pycache__", "target":
		return true
	}
	return false
}

// maxFileSize 单文件最大索引大小（10MB）
const maxFileSize = 10 * 1024 * 1024

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
				// 限制单文件大小
				if info.Size() > maxFileSize {
					slog.Warn("文件过大，跳过", "path", path, "size", info.Size())
					break
				}
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

// WalkFiles 流式遍历文件，对每个文件调用 callback 处理后立即释放内容
// 避免将所有文件内容加载到内存
func WalkFiles(root string, extensions []string, callback func(relPath string, content []byte) error) error {
	extMap := make(map[string]bool, len(extensions))
	for _, ext := range extensions {
		extMap[ext] = true
	}

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
		if !extMap[ext] {
			return nil
		}

		// 限制单文件大小（10MB）
		if info.Size() > 10*1024*1024 {
			slog.Warn("文件过大，跳过", "path", path, "size", info.Size())
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			slog.Warn("读取文件失败", "path", path, "err", readErr)
			return nil
		}

		relPath, relErr := filepath.Rel(root, path)
		if relErr != nil {
			relPath = path
		}

		return callback(relPath, content)
	})
}

// ScanSpecificFiles 仅扫描指定的文件列表（相对路径），避免全量扫描整个项目
func ScanSpecificFiles(root string, filePaths []string) ([]document, error) {
	var docs []document
	for _, relPath := range filePaths {
		fullPath := filepath.Join(root, relPath)

		info, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				// 文件已被删除，跳过
				continue
			}
			slog.Warn("获取文件信息失败", "path", fullPath, "err", err)
			continue
		}
		if info.IsDir() {
			continue
		}

		// 限制单文件大小（10MB）
		if info.Size() > 10*1024*1024 {
			slog.Warn("文件过大，跳过", "path", fullPath, "size", info.Size())
			continue
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			slog.Warn("读取文件失败", "path", fullPath, "err", err)
			continue
		}

		docs = append(docs, document{FilePath: relPath, Content: string(content)})
	}
	return docs, nil
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
	const embeddingBatchSize = 100
	var allVectors [][]float32
	var allTexts []string
	var allIDs []string
	var allMetadatas []map[string]interface{}

	chunkIdx := 0
	totalChunks := 0

	// 用于批量 embedding 的临时缓冲
	var embedBatch []string
	var embedChunks []types.CodeChunk
	var embedFiles []string

	flushEmbedBatch := func() error {
		if len(embedBatch) == 0 {
			return nil
		}
		vectors, err := embedding.GetBatchEmbeddings(ctx, cfg, embedBatch)
		if err != nil {
			// 批量失败，逐条重试以保证最大成功数
			slog.Warn("批量向量化失败，逐条重试", "err", err)
			for j, text := range embedBatch {
				vector, singleErr := embedding.GetEmbedding(ctx, cfg, text)
				if singleErr != nil {
					slog.Warn("向量化失败", "file", embedFiles[j], "err", singleErr)
					continue
				}
				chunkIdx++
				totalChunks++
				allIDs = append(allIDs, fmt.Sprintf("doc_%d", chunkIdx))
				allTexts = append(allTexts, text)
				allVectors = append(allVectors, vector)
				allMetadatas = append(allMetadatas, embedChunks[j].Metadata)
			}
		} else {
			for j, vector := range vectors {
				chunkIdx++
				totalChunks++
				allIDs = append(allIDs, fmt.Sprintf("doc_%d", chunkIdx))
				allTexts = append(allTexts, embedBatch[j])
				allVectors = append(allVectors, vector)
				allMetadatas = append(allMetadatas, embedChunks[j].Metadata)
			}
		}
		embedBatch = embedBatch[:0]
		embedChunks = embedChunks[:0]
		embedFiles = embedFiles[:0]

		// 如果累积的向量达到 vdb 批量大小，刷入存储
		if len(allVectors) >= batchSize {
			if err := flushBatch(ctx, vdb, allIDs, allTexts, allVectors, allMetadatas); err != nil {
				return err
			}
			allIDs = allIDs[:0]
			allTexts = allTexts[:0]
			allVectors = allVectors[:0]
			allMetadatas = allMetadatas[:0]
		}
		return nil
	}

	for _, doc := range docs {
		lang := structure.DetectLanguage(doc.FilePath)
		chunks := structure.SplitByStructure(doc.Content, lang, doc.FilePath, cfg.MaxChunkSize)

		// 构建倒排索引
		if invIndex != nil {
			invIndex.BuildFromChunks(chunks, doc.FilePath)
		}

		for _, chunk := range chunks {
			embedBatch = append(embedBatch, chunk.Content)
			embedChunks = append(embedChunks, chunk)
			embedFiles = append(embedFiles, doc.FilePath)

			if len(embedBatch) >= embeddingBatchSize {
				if err := flushEmbedBatch(); err != nil {
					return nil, err
				}
			}
		}
	}

	// 刷新剩余的 embedding 批次
	if err := flushEmbedBatch(); err != nil {
		return nil, err
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
