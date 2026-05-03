package main

import (
	"os"
	"strconv"
	"strings"
)

// Config MCP 服务器全局配置
type Config struct {
	// Ollama 嵌入模型
	OllamaURL      string
	OllamaModel    string
	EmbeddingDim   int

	// Zilliz Cloud 向量数据库
	ZillizURI      string
	ZillizToken    string
	CollectionName string

	// 索引配置
	ScanExtensions []string
	ChunkSize      int
	MaxChunkSize   int
	AutoIndex      bool
	ProjectPath    string
	IndexStatePath string
}

// LoadConfig 从环境变量加载配置，提供合理默认值
func LoadConfig() Config {
	return Config{
		OllamaURL:      getEnv("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:    getEnv("OLLAMA_EMBED_MODEL", "nomic-embed-text:latest"),
		EmbeddingDim:   getEnvInt("EMBEDDING_DIM", 768),

		ZillizURI:      getEnv("ZILLIZ_URI", ""),
		ZillizToken:    getEnv("ZILLIZ_TOKEN", ""),
		CollectionName: getEnv("COLLECTION_NAME", "code-context"),

		ScanExtensions: getEnvSlice("SCAN_EXTENSIONS", []string{".go", ".vue", ".js", ".ts", ".py", ".md"}),
		ChunkSize:      getEnvInt("CHUNK_SIZE", 800),
		MaxChunkSize:   getEnvInt("MAX_CHUNK_SIZE", 1500),
		AutoIndex:      getEnvBool("AUTO_INDEX", true),
		ProjectPath:    getEnv("PROJECT_PATH", ""),
		IndexStatePath: getEnv("INDEX_STATE_PATH", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	switch strings.ToLower(v) {
	case "true", "1":
		return true
	case "false", "0":
		return false
	default:
		return fallback
	}
}

func getEnvSlice(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}
