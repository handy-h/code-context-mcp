package search

import "context"

// VectorStore 向量存储后端接口，供索引构建和语义搜索使用。
type VectorStore interface {
	DropCollection(ctx context.Context) error
	Close()
	HasCollection(ctx context.Context) (bool, error)
	DeleteByFile(ctx context.Context, filePath string) error
	EnsureCollection(ctx context.Context) error
	Insert(ctx context.Context, ids []string, texts []string, vectors [][]float32, metadatas []map[string]interface{}) error
	Search(ctx context.Context, queryVector []float32, topK int) ([]CodeSearchResult, error)
}

// CodeSearchResult 语义搜索结果
type CodeSearchResult struct {
	File  string  `json:"file"`
	Text  string  `json:"text"`
	Score float32 `json:"score"`
}
