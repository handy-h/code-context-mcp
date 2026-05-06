package embedding

import "github.com/handy-h/code-context-mcp/internal/config"

// ollamaRequest Ollama embeddings API 请求体
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// ollamaResponse Ollama embeddings API 响应体
type ollamaResponse struct {
	Model     string    `json:"model"`
	Embedding []float32 `json:"embedding"`
}

// GetEmbedding 获取文本的嵌入向量
func GetEmbedding(cfg config.Config, text string) ([]float32, error) {
	provider, err := NewEmbeddingProvider(cfg)
	if err != nil {
		return nil, err
	}
	return provider.GetEmbedding(text)
}
