package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/handy-h/code-context-mcp/internal/config"
)

// EmbeddingProvider 嵌入模型提供者接口
type EmbeddingProvider interface {
	// GetEmbedding 获取文本的嵌入向量
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
	// GetBatchEmbeddings 批量获取文本的嵌入向量
	GetBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
	// GetDimension 获取嵌入向量的维度
	GetDimension() int
}

// 编译期接口实现检查
var (
	_ EmbeddingProvider = (*OllamaProvider)(nil)
	_ EmbeddingProvider = (*OpenAIProvider)(nil)
	_ EmbeddingProvider = (*GeminiProvider)(nil)
)

// OllamaProvider Ollama 嵌入模型提供者
type OllamaProvider struct {
	url    string
	model  string
	dim    int
	client *http.Client
}

// NewOllamaProvider 创建 Ollama 提供者
func NewOllamaProvider(url, model string, dim int) *OllamaProvider {
	return &OllamaProvider{
		url:    strings.TrimSuffix(url, "/"),
		model:  model,
		dim:    dim,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// GetEmbedding 实现 EmbeddingProvider 接口
func (p *OllamaProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	reqBody := ollamaRequest{
		Model:      p.model,
		Prompt:     text,
		Dimensions: p.dim,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("JSON 序列化失败: %w", err)
	}

	url := p.url + "/api/embeddings"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Ollama 请求失败（请确认 Ollama 已运行）: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama 返回错误状态码: %d, 响应: %s", resp.StatusCode, string(bodyBytes))
	}

	var result ollamaResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w, 响应体: %s", err, string(bodyBytes))
	}

	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("Ollama 返回空向量, 响应: %s", string(bodyBytes))
	}

	return result.Embedding, nil
}

// GetDimension 返回向量维度
func (p *OllamaProvider) GetDimension() int {
	return p.dim
}

// GetBatchEmbeddings 批量获取文本的嵌入向量（Ollama 不支持批量 API，逐个调用）
func (p *OllamaProvider) GetBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, 0, len(texts))
	for _, text := range texts {
		vec, err := p.GetEmbedding(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("批量 embedding 在第 %d 条文本失败: %w", len(results), err)
		}
		results = append(results, vec)
	}
	return results, nil
}

// OpenAIProvider OpenAI 兼容 API 嵌入模型提供者
type OpenAIProvider struct {
	baseURL string
	model   string
	apiKey  string
	dim     int
	client  *http.Client
}

// NewOpenAIProvider 创建 OpenAI 提供者
func NewOpenAIProvider(baseURL, model, apiKey string, dim int) *OpenAIProvider {
	return &OpenAIProvider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		model:   model,
		apiKey:  apiKey,
		dim:     dim,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// openaiEmbeddingRequest OpenAI embeddings API 请求体
type openaiEmbeddingRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

// openaiEmbeddingResponse OpenAI embeddings API 响应体
type openaiEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// GetEmbedding 实现 EmbeddingProvider 接口
func (p *OpenAIProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("OpenAI API Key 未配置")
	}

	reqBody := openaiEmbeddingRequest{
		Model:      p.model,
		Input:      []string{text},
		Dimensions: p.dim,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("JSON 序列化失败: %w", err)
	}

	url := p.baseURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API 请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API 返回错误状态码: %d, 响应: %s", resp.StatusCode, string(bodyBytes))
	}

	var result openaiEmbeddingResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w, 响应体: %s", err, string(bodyBytes))
	}

	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("OpenAI API 返回空向量, 响应: %s", string(bodyBytes))
	}

	return result.Data[0].Embedding, nil
}

// GetDimension 返回向量维度
func (p *OpenAIProvider) GetDimension() int {
	return p.dim
}

// GetBatchEmbeddings 批量获取文本的嵌入向量（OpenAI 支持真正的批量请求）
func (p *OpenAIProvider) GetBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("OpenAI API Key 未配置")
	}

	reqBody := openaiEmbeddingRequest{
		Model:      p.model,
		Input:      texts,
		Dimensions: p.dim,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("JSON 序列化失败: %w", err)
	}

	url := p.baseURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API 请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API 返回错误状态码: %d, 响应: %s", resp.StatusCode, string(bodyBytes))
	}

	var result openaiEmbeddingResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w, 响应体: %s", err, string(bodyBytes))
	}

	// 按 index 排序以保持顺序
	sort.Slice(result.Data, func(i, j int) bool {
		return result.Data[i].Index < result.Data[j].Index
	})

	results := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		results[i] = d.Embedding
	}
	return results, nil
}

// GeminiProvider Google Gemini 嵌入模型提供者
type GeminiProvider struct {
	baseURL string
	model   string
	apiKey  string
	dim     int
	client  *http.Client
}

// NewGeminiProvider 创建 Gemini 提供者
func NewGeminiProvider(baseURL, model, apiKey string, dim int) *GeminiProvider {
	return &GeminiProvider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		model:   model,
		apiKey:  apiKey,
		dim:     dim,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// GetEmbedding 实现 EmbeddingProvider 接口
func (p *GeminiProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("Gemini API Key 未配置")
	}

	// Gemini Embeddings API 请求格式
	reqBody := map[string]interface{}{
		"model": fmt.Sprintf("models/%s", p.model),
		"content": map[string]interface{}{
			"parts": []map[string]interface{}{
				{
					"text": text,
				},
			},
		},
		"taskType": "RETRIEVAL_DOCUMENT",
	}

	// 如果指定了维度，添加 output_dimensionality 参数
	if p.dim > 0 {
		reqBody["output_dimensionality"] = p.dim
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("JSON 序列化失败: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:embedContent", p.baseURL, p.model)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Gemini API 请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// 尝试解析错误信息
		var errResp map[string]interface{}
		if json.Unmarshal(bodyBytes, &errResp) == nil {
			if errorObj, ok := errResp["error"].(map[string]interface{}); ok {
				if message, ok := errorObj["message"].(string); ok {
					return nil, fmt.Errorf("Gemini API 错误: %s (状态码: %d)", message, resp.StatusCode)
				}
			}
		}
		return nil, fmt.Errorf("Gemini API 返回错误状态码: %d, 响应: %s", resp.StatusCode, string(bodyBytes))
	}

	// 解析响应 - 响应格式是 {"embedding": {"values": [...]}}
	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w, 响应体: %s", err, string(bodyBytes))
	}

	// 提取嵌入向量
	var embedding []float32
	if embeddingObj, ok := result["embedding"].(map[string]interface{}); ok {
		if values, ok := embeddingObj["values"].([]interface{}); ok {
			embedding = make([]float32, len(values))
			for i, v := range values {
				if f, ok := v.(float64); ok {
					embedding[i] = float32(f)
				} else {
					return nil, fmt.Errorf("索引 %d 处的 embedding 值无效: %v", i, v)
				}
			}
		}
	}

	if len(embedding) == 0 {
		return nil, fmt.Errorf("Gemini API 返回空或无效的向量, 响应: %s", string(bodyBytes))
	}

	// 检查向量维度
	if p.dim > 0 && len(embedding) != p.dim {
		return nil, fmt.Errorf("embedding 维度不匹配: 期望 %d, 实际 %d", p.dim, len(embedding))
	}

	return embedding, nil
}

// GetDimension 返回向量维度
func (p *GeminiProvider) GetDimension() int {
	return p.dim
}

// GetBatchEmbeddings 批量获取文本的嵌入向量（Gemini embedContent 是单文本 API，逐个调用）
func (p *GeminiProvider) GetBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, 0, len(texts))
	for _, text := range texts {
		vec, err := p.GetEmbedding(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("批量 embedding 在第 %d 条文本失败: %w", len(results), err)
		}
		results = append(results, vec)
	}
	return results, nil
}

// NewEmbeddingProvider 创建嵌入模型提供者
func NewEmbeddingProvider(cfg config.Config) (EmbeddingProvider, error) {
	switch cfg.EmbeddingProvider {
	case config.ProviderOllama:
		return NewOllamaProvider(cfg.OllamaURL, cfg.OllamaModel, cfg.EmbeddingDim), nil
	case config.ProviderOpenAI:
		return NewOpenAIProvider(cfg.OpenAIBaseURL, cfg.OpenAIModel, cfg.OpenAIAPIKey, cfg.EmbeddingDim), nil
	case config.ProviderGemini:
		return NewGeminiProvider(cfg.GeminiBaseURL, cfg.GeminiModel, cfg.GeminiAPIKey, cfg.EmbeddingDim), nil
	default:
		return nil, fmt.Errorf("不支持的 embedding 提供者: %s", cfg.EmbeddingProvider)
	}
}
