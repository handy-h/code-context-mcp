package search

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/handy-h/code-context-mcp/internal/config"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// VectorDB 封装 Zilliz Cloud 向量数据库操作
type VectorDB struct {
	client client.Client
	cfg    config.Config
}

// CodeSearchResult 语义搜索结果
type CodeSearchResult struct {
	File  string  `json:"file"`
	Text  string  `json:"text"`
	Score float32 `json:"score"`
}

// NewVectorDB 创建向量数据库连接
func NewVectorDB(ctx context.Context, cfg config.Config) (*VectorDB, error) {
	c, err := client.NewClient(ctx, client.Config{
		Address: cfg.ZillizURI,
		APIKey:  cfg.ZillizToken,
	})
	if err != nil {
		return nil, fmt.Errorf("连接Zilliz失败: %v", err)
	}
	return &VectorDB{client: c, cfg: cfg}, nil
}

// DropCollection 删除集合（重建索引时清理旧数据）
func (v *VectorDB) DropCollection(ctx context.Context) error {
	return v.client.DropCollection(ctx, v.cfg.CollectionName)
}

// Close 关闭连接
func (v *VectorDB) Close() {
	v.client.Close()
}

// HasCollection 检查集合是否存在
func (v *VectorDB) HasCollection(ctx context.Context) (bool, error) {
	return v.client.HasCollection(ctx, v.cfg.CollectionName)
}

// DeleteByFile 按文件路径删除向量数据
func (v *VectorDB) DeleteByFile(ctx context.Context, filePath string) error {
	has, err := v.client.HasCollection(ctx, v.cfg.CollectionName)
	if err != nil || !has {
		return nil // 集合不存在，无需删除
	}

	// 使用 metadata.file 过滤表达式删除
	expr := fmt.Sprintf(`metadata["file"] == "%s"`, filePath)
	if err := v.client.Delete(ctx, v.cfg.CollectionName, "", expr); err != nil {
		return fmt.Errorf("按文件删除向量失败: %v", err)
	}
	return nil
}

// EnsureCollection 确保集合存在，不存在则创建
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

	// 创建索引
	idx := entity.NewGenericIndex("embedding_idx", entity.AUTOINDEX, map[string]string{
		"metric_type": string(entity.COSINE),
	})
	if err := v.client.CreateIndex(ctx, v.cfg.CollectionName, "embedding", idx, false); err != nil {
		return fmt.Errorf("创建索引失败: %v", err)
	}

	// 加载集合到内存（Zilliz Serverless 需要显式加载才能搜索）
	if err := v.client.LoadCollection(ctx, v.cfg.CollectionName, false); err != nil {
		return fmt.Errorf("加载集合失败: %v", err)
	}

	log.Printf("集合 %s 创建并加载完成", v.cfg.CollectionName)
	return nil
}

// Insert 批量插入向量数据
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

// Search 语义搜索：query向量 → 向量搜索 → 返回结果
func (v *VectorDB) Search(ctx context.Context, queryVector []float32, topK int) ([]CodeSearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	// 确保集合已加载到内存（Zilliz Serverless 需要显式加载）
	if err := v.client.LoadCollection(ctx, v.cfg.CollectionName, false); err != nil {
		return nil, fmt.Errorf("加载集合失败: %v", err)
	}

	// 创建搜索参数
	sp, err := entity.NewIndexAUTOINDEXSearchParam(1)
	if err != nil {
		return nil, fmt.Errorf("创建搜索参数失败: %v", err)
	}

	results, err := v.client.Search(
		ctx, v.cfg.CollectionName,
		[]string{},                   // partition names
		"",                           // expression filter
		[]string{"text", "metadata"}, // output fields
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
			sr := CodeSearchResult{
				Score: results[0].Scores[i],
			}
			// 提取 text 字段
			textCol := results[0].Fields.GetColumn("text")
			if textCol != nil {
				if vc, ok := textCol.(*entity.ColumnVarChar); ok && i < len(vc.Data()) {
					sr.Text = vc.Data()[i]
				}
			}
			// 提取 metadata 中的 file 字段
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
