package embedding

import (
	"context"
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
	providerOnce   sync.Once
	providerErr    error
)

const (
	maxRetries     = 3
	baseRetryDelay = 1 * time.Second
)

// getSharedProvider 获取或创建全局 Provider 实例（线程安全，仅初始化一次）
func getSharedProvider(cfg config.Config) (EmbeddingProvider, error) {
	providerOnce.Do(func() {
		sharedProvider, providerErr = NewEmbeddingProvider(cfg)
	})
	return sharedProvider, providerErr
}

// GetEmbedding 获取文本的嵌入向量
// 复用全局 Provider 实例，避免每次调用都新建 HTTP Client
// 内置指数退避重试机制，处理瞬态网络错误和限流
func GetEmbedding(ctx context.Context, cfg config.Config, text string) ([]float32, error) {
	provider, err := getSharedProvider(cfg)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// 检查 context 是否已取消
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("embedding 请求已取消: %w", err)
		}

		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseRetryDelay
			slog.Debug("重试 embedding 请求", "attempt", attempt+1, "delay", delay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, fmt.Errorf("embedding 重试等待已取消: %w", ctx.Err())
			}
		}
		result, err := provider.GetEmbedding(ctx, text)
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
func GetBatchEmbeddings(ctx context.Context, cfg config.Config, texts []string) ([][]float32, error) {
	provider, err := getSharedProvider(cfg)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("批量 embedding 请求已取消: %w", err)
	}
	return provider.GetBatchEmbeddings(ctx, texts)
}
