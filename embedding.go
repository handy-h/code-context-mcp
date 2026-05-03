package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ollamaRequest Ollama embeddings API 请求体
type ollamaRequest struct {
	Model string `json:"model"`
	Prompt string `json:"prompt"`
}

// ollamaResponse Ollama embeddings API 响应体
type ollamaResponse struct {
	Model     string    `json:"model"`
	Embedding []float32 `json:"embedding"`
}

// GetEmbedding 调用 Ollama 本地嵌入模型获取向量
func GetEmbedding(cfg Config, text string) ([]float32, error) {
	reqBody := ollamaRequest{
		Model:  cfg.OllamaModel,
		Prompt: text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("json serialization failed: %v", err)
	}

	url := cfg.OllamaURL + "/api/embeddings"
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
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
