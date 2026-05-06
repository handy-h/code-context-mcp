package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// EmbeddingProvider 嵌入模型提供者接口
type EmbeddingProvider interface {
	// GetEmbedding 获取文本的嵌入向量
	GetEmbedding(text string) ([]float32, error)
	// GetDimension 获取嵌入向量的维度
	GetDimension() int
}

// OllamaProvider Ollama 嵌入模型提供者
type OllamaProvider struct {
	url   string
	model string
	dim   int
}

// NewOllamaProvider 创建 Ollama 提供者
func NewOllamaProvider(url, model string, dim int) *OllamaProvider {
	return &OllamaProvider{
		url:   strings.TrimSuffix(url, "/"),
		model: model,
		dim:   dim,
	}
}

// GetEmbedding 实现 EmbeddingProvider 接口
func (p *OllamaProvider) GetEmbedding(text string) ([]float32, error) {
	reqBody := ollamaRequest{
		Model:  p.model,
		Prompt: text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("json serialization failed: %v", err)
	}

	url := p.url + "/api/embeddings"
	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed (please confirm Ollama is running): %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned error status code: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	var result ollamaResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("json parsing failed: %v, response body: %s", err, string(bodyBytes))
	}

	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("ollama returned empty vector, response: %s", string(bodyBytes))
	}

	return result.Embedding, nil
}

// GetDimension 返回向量维度
func (p *OllamaProvider) GetDimension() int {
	return p.dim
}

// OpenAIProvider OpenAI 兼容 API 嵌入模型提供者
type OpenAIProvider struct {
	baseURL string
	model   string
	apiKey  string
	dim     int
}

// NewOpenAIProvider 创建 OpenAI 提供者
func NewOpenAIProvider(baseURL, model, apiKey string, dim int) *OpenAIProvider {
	return &OpenAIProvider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		model:   model,
		apiKey:  apiKey,
		dim:     dim,
	}
}

// openaiEmbeddingRequest OpenAI embeddings API 请求体
type openaiEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
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
func (p *OpenAIProvider) GetEmbedding(text string) ([]float32, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	reqBody := openaiEmbeddingRequest{
		Model: p.model,
		Input: []string{text},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("json serialization failed: %v", err)
	}

	url := p.baseURL + "/embeddings"
	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API returned error status code: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	var result openaiEmbeddingResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("json parsing failed: %v, response body: %s", err, string(bodyBytes))
	}

	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("OpenAI API returned empty vector, response: %s", string(bodyBytes))
	}

	return result.Data[0].Embedding, nil
}

// GetDimension 返回向量维度
func (p *OpenAIProvider) GetDimension() int {
	return p.dim
}

// GeminiProvider Google Gemini 嵌入模型提供者
type GeminiProvider struct {
	baseURL string
	model   string
	apiKey  string
	dim     int
}

// NewGeminiProvider 创建 Gemini 提供者
func NewGeminiProvider(baseURL, model, apiKey string, dim int) *GeminiProvider {
	return &GeminiProvider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		model:   model,
		apiKey:  apiKey,
		dim:     dim,
	}
}

// geminiEmbeddingRequest Gemini embeddings API 请求体
type geminiEmbeddingRequest struct {
	Model     string                       `json:"model"`
	Content   geminiEmbeddingContent       `json:"content"`
	TaskType  string                       `json:"taskType,omitempty"`
	Title     string                       `json:"title,omitempty"`
	OutputDimensionality int               `json:"outputDimensionality,omitempty"`
}

type geminiEmbeddingContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

// geminiEmbeddingResponse Gemini embeddings API 响应体
type geminiEmbeddingResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
}

// GetEmbedding 实现 EmbeddingProvider 接口
func (p *GeminiProvider) GetEmbedding(text string) ([]float32, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("Gemini API key is required")
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
		// 可选参数
		"task_type": "RETRIEVAL_DOCUMENT", // 或 "RETRIEVAL_QUERY"
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("json serialization failed: %v", err)
	}

	// Gemini API 使用 URL 参数传递 API 密钥
	// 格式: https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-001:embedContent?key=API_KEY
	url := fmt.Sprintf("%s/models/%s:embedContent?key=%s", p.baseURL, p.model, p.apiKey)
	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Gemini API request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}



	if resp.StatusCode != http.StatusOK {
		// 尝试解析错误信息
		var errResp map[string]interface{}
		if json.Unmarshal(bodyBytes, &errResp) == nil {
			if errorObj, ok := errResp["error"].(map[string]interface{}); ok {
				if message, ok := errorObj["message"].(string); ok {
					return nil, fmt.Errorf("Gemini API error: %s (status: %d)", message, resp.StatusCode)
				}
			}
		}
		return nil, fmt.Errorf("Gemini API returned error status code: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	// 解析响应 - 根据测试输出，响应格式是 {"embedding": {"values": [...]}}
	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("json parsing failed: %v, response body: %s", err, string(bodyBytes))
	}

	// 提取嵌入向量
	var embedding []float32
	if embeddingObj, ok := result["embedding"].(map[string]interface{}); ok {
		if values, ok := embeddingObj["values"].([]interface{}); ok {
			embedding = make([]float32, len(values))
			for i, v := range values {
				// JSON 数字默认解析为 float64
				if f, ok := v.(float64); ok {
					embedding[i] = float32(f)
				} else {
					return nil, fmt.Errorf("invalid embedding value at index %d: %v", i, v)
				}
			}
		}
	}

	if len(embedding) == 0 {
		return nil, fmt.Errorf("Gemini API returned empty or invalid vector, response: %s", string(bodyBytes))
	}

	// 检查向量维度
	if p.dim > 0 && len(embedding) != p.dim {
		return nil, fmt.Errorf("embedding dimension mismatch: expected %d, got %d", p.dim, len(embedding))
	}

	return embedding, nil
}

// GetDimension 返回向量维度
func (p *GeminiProvider) GetDimension() int {
	return p.dim
}

// NewEmbeddingProvider 创建嵌入模型提供者
func NewEmbeddingProvider(cfg Config) (EmbeddingProvider, error) {
	switch cfg.EmbeddingProvider {
	case ProviderOllama:
		return NewOllamaProvider(cfg.OllamaURL, cfg.OllamaModel, cfg.EmbeddingDim), nil
	case ProviderOpenAI:
		return NewOpenAIProvider(cfg.OpenAIBaseURL, cfg.OpenAIModel, cfg.OpenAIAPIKey, cfg.EmbeddingDim), nil
	case ProviderGemini:
		return NewGeminiProvider(cfg.GeminiBaseURL, cfg.GeminiModel, cfg.GeminiAPIKey, cfg.EmbeddingDim), nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", cfg.EmbeddingProvider)
	}
}