package embedding

import (
	"sync"

	"github.com/handy-h/code-context-mcp/internal/config"
)

// ollamaRequest Ollama embeddings API 请求体
type ollamaRequest struct {
	Model      string `json:"model"`
	Prompt     string `json:"prompt"`
	Dimensions int    `json:"dimensions,omitempty"`
}

// ollamaResponse Ollama embeddings API 响应体
type ollamaResponse struct {
	Model     string    `json:"model"`
	Embedding []float32 `json:"embedding"`
}

var (
	sharedProvider EmbeddingProvider
	sharedMu       sync.Mutex
)

// GetEmbedding 获取文本的嵌入向量
// 复用全局 Provider 实例，避免每次调用都新建 HTTP Client
func GetEmbedding(cfg config.Config, text string) ([]float32, error) {
	sharedMu.Lock()
	if sharedProvider == nil {
		var err error
		sharedProvider, err = NewEmbeddingProvider(cfg)
		if err != nil {
			sharedMu.Unlock()
			return nil, err
		}
	}
	sharedMu.Unlock()
	return sharedProvider.GetEmbedding(text)
}
