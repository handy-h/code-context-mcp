package search

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/handy-h/code-context-mcp/internal/config"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// VectorStore is the storage backend used by indexing and semantic search.
type VectorStore interface {
	DropCollection(ctx context.Context) error
	Close()
	HasCollection(ctx context.Context) (bool, error)
	DeleteByFile(ctx context.Context, filePath string) error
	EnsureCollection(ctx context.Context) error
	Insert(ctx context.Context, ids []string, texts []string, vectors [][]float32, metadatas []map[string]interface{}) error
	Search(ctx context.Context, queryVector []float32, topK int) ([]CodeSearchResult, error)
}

// VectorDB wraps a Zilliz Cloud vector database client.
type VectorDB struct {
	client client.Client
	cfg    config.Config
}

// CodeSearchResult describes a semantic search result.
type CodeSearchResult struct {
	File  string  `json:"file"`
	Text  string  `json:"text"`
	Score float32 `json:"score"`
}

// NewVectorDB creates the configured vector store backend.
func NewVectorDB(ctx context.Context, cfg config.Config) (VectorStore, error) {
	switch cfg.VectorStore {
	case config.VectorStoreZilliz:
		return NewZillizVectorDB(ctx, cfg)
	case config.VectorStoreLocalJSONL:
		return NewLocalJSONLStore(cfg.VectorStorePath)
	default:
		return nil, fmt.Errorf("unsupported vector store: %s", cfg.VectorStore)
	}
}

// NewZillizVectorDB creates a Zilliz-backed vector store.
func NewZillizVectorDB(ctx context.Context, cfg config.Config) (*VectorDB, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	c, err := client.NewClient(ctx, client.Config{
		Address: cfg.ZillizURI,
		APIKey:  cfg.ZillizToken,
	})
	if err != nil {
		return nil, fmt.Errorf("连接Zilliz失败: %v", err)
	}
	return &VectorDB{client: c, cfg: cfg}, nil
}

// DropCollection deletes the backing collection.
func (v *VectorDB) DropCollection(ctx context.Context) error {
	return v.client.DropCollection(ctx, v.cfg.CollectionName)
}

// Close closes the client.
func (v *VectorDB) Close() {
	v.client.Close()
}

// HasCollection checks whether the backing collection exists.
func (v *VectorDB) HasCollection(ctx context.Context) (bool, error) {
	return v.client.HasCollection(ctx, v.cfg.CollectionName)
}

// DeleteByFile removes vectors whose metadata.file matches filePath.
func (v *VectorDB) DeleteByFile(ctx context.Context, filePath string) error {
	has, err := v.client.HasCollection(ctx, v.cfg.CollectionName)
	if err != nil || !has {
		return nil
	}

	expr := fmt.Sprintf(`metadata["file"] == "%s"`, filePath)
	if err := v.client.Delete(ctx, v.cfg.CollectionName, "", expr); err != nil {
		return fmt.Errorf("按文件删除向量失败: %v", err)
	}
	return nil
}

// EnsureCollection ensures the collection exists and is loaded.
func (v *VectorDB) EnsureCollection(ctx context.Context) error {
	has, err := v.client.HasCollection(ctx, v.cfg.CollectionName)
	if err != nil {
		return fmt.Errorf("检查集合失败: %v", err)
	}
	if has {
		return nil
	}

	log.Printf("创建集合 %s (dim=%d)...", v.cfg.CollectionName, v.cfg.EmbeddingDim)

	schema := entity.NewSchema().
		WithName(v.cfg.CollectionName).
		WithDescription("Code context for AI coding assistant").
		WithField(
			entity.NewField().WithName("id").WithIsPrimaryKey(true).
				WithDataType(entity.FieldTypeVarChar).WithMaxLength(100),
		).
		WithField(
			entity.NewField().WithName("text").WithDataType(entity.FieldTypeVarChar).WithMaxLength(65535),
		).
		WithField(
			entity.NewField().WithName("embedding").WithDataType(entity.FieldTypeFloatVector).
				WithDim(int64(v.cfg.EmbeddingDim)),
		).
		WithField(
			entity.NewField().WithName("metadata").WithDataType(entity.FieldTypeJSON),
		)

	if err := v.client.CreateCollection(ctx, schema, 2); err != nil {
		return fmt.Errorf("创建集合失败: %v", err)
	}

	idx := entity.NewGenericIndex("embedding_idx", entity.AUTOINDEX, map[string]string{
		"metric_type": string(entity.COSINE),
	})
	if err := v.client.CreateIndex(ctx, v.cfg.CollectionName, "embedding", idx, false); err != nil {
		return fmt.Errorf("创建索引失败: %v", err)
	}

	if err := v.client.LoadCollection(ctx, v.cfg.CollectionName, false); err != nil {
		return fmt.Errorf("加载集合失败: %v", err)
	}

	log.Printf("集合 %s 创建并加载完成", v.cfg.CollectionName)
	return nil
}

// Insert inserts vectors into the collection.
func (v *VectorDB) Insert(ctx context.Context, ids []string, texts []string, vectors [][]float32, metadatas []map[string]interface{}) error {
	metaBytes := make([][]byte, len(metadatas))
	for i, m := range metadatas {
		b, err := json.Marshal(m)
		if err != nil {
			metaBytes[i] = []byte("{}")
		} else {
			metaBytes[i] = b
		}
	}

	_, err := v.client.Insert(ctx, v.cfg.CollectionName, "",
		entity.NewColumnVarChar("id", ids),
		entity.NewColumnVarChar("text", texts),
		entity.NewColumnFloatVector("embedding", v.cfg.EmbeddingDim, vectors),
		entity.NewColumnJSONBytes("metadata", metaBytes),
	)
	return err
}

// Search performs semantic search against the vector store.
func (v *VectorDB) Search(ctx context.Context, queryVector []float32, topK int) ([]CodeSearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	if err := v.client.LoadCollection(ctx, v.cfg.CollectionName, false); err != nil {
		return nil, fmt.Errorf("加载集合失败: %v", err)
	}

	sp, err := entity.NewIndexAUTOINDEXSearchParam(1)
	if err != nil {
		return nil, fmt.Errorf("创建搜索参数失败: %v", err)
	}

	results, err := v.client.Search(
		ctx, v.cfg.CollectionName,
		[]string{},
		"",
		[]string{"text", "metadata"},
		[]entity.Vector{entity.FloatVector(queryVector)},
		"embedding",
		entity.COSINE,
		topK,
		sp,
	)
	if err != nil {
		return nil, fmt.Errorf("向量搜索失败: %v", err)
	}

	var searchResults []CodeSearchResult
	if len(results) > 0 {
		for i := 0; i < len(results[0].Scores); i++ {
			sr := CodeSearchResult{Score: results[0].Scores[i]}
			textCol := results[0].Fields.GetColumn("text")
			if textCol != nil {
				if vc, ok := textCol.(*entity.ColumnVarChar); ok && i < len(vc.Data()) {
					sr.Text = vc.Data()[i]
				}
			}
			metaCol := results[0].Fields.GetColumn("metadata")
			if metaCol != nil {
				if jc, ok := metaCol.(*entity.ColumnJSONBytes); ok && i < len(jc.Data()) {
					var meta map[string]interface{}
					if json.Unmarshal(jc.Data()[i], &meta) == nil {
						if f, ok := meta["file"].(string); ok {
							sr.File = f
						}
					}
				}
			}
			searchResults = append(searchResults, sr)
		}
	}

	return searchResults, nil
}
