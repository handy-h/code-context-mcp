package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
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
		return nil, fmt.Errorf("JSON序列化失败: %v", err)
	}

	url := cfg.OllamaURL + "/api/embeddings"
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Ollama请求失败(请确认Ollama已启动): %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama返回错误状态码: %d, 响应: %s", resp.StatusCode, string(bodyBytes))
	}

	var result ollamaResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %v, 响应体: %s", err, string(bodyBytes))
	}

	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("Ollama返回空向量, 响应: %s", string(bodyBytes))
	}

	return result.Embedding, nil
}
