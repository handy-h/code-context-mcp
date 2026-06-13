package embedding

import (
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

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

const (
	maxRetries     = 3
	baseRetryDelay = 1 * time.Second
)

// GetEmbedding 获取文本的嵌入向量
// 复用全局 Provider 实例，避免每次调用都新建 HTTP Client
// 内置指数退避重试机制，处理瞬态网络错误和限流
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

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseRetryDelay
			slog.Debug("重试 embedding 请求", "attempt", attempt+1, "delay", delay)
			time.Sleep(delay)
		}
		result, err := sharedProvider.GetEmbedding(text)
		if err == nil {
			return result, nil
		}
		lastErr = err
		// 仅对可能的瞬态错误重试（网络错误、5xx、429）
		if !isRetryableError(err.Error()) {
			return nil, err
		}
		slog.Warn("embedding 请求失败，将重试", "attempt", attempt+1, "err", err)
	}
	return nil, fmt.Errorf("embedding 请求在 %d 次重试后仍然失败: %w", maxRetries, lastErr)
}

// isRetryableError 判断错误是否值得重试
func isRetryableError(errMsg string) bool {
	retryableKeywords := []string{
		"connection refused", "connection reset", "timeout",
		"EOF", "broken pipe", "TLS handshake",
		"dial tcp", "no such host",
		"status code: 429", "status code: 500", "status code: 502",
		"status code: 503", "status code: 504",
	}
	for _, kw := range retryableKeywords {
		if strings.Contains(errMsg, kw) {
			return true
		}
	}
	return false
}

// GetBatchEmbeddings 批量获取文本的嵌入向量
func GetBatchEmbeddings(cfg config.Config, texts []string) ([][]float32, error) {
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
	return sharedProvider.GetBatchEmbeddings(texts)
}
