package main

import (
	"fmt"
	"os"
	"regexp"
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
	collectionName := normalizeCollectionName(getEnv("COLLECTION_NAME", "code_context"))

	return Config{
		OllamaURL:      getEnv("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:    getEnv("OLLAMA_EMBED_MODEL", "nomic-embed-text:latest"),
		EmbeddingDim:   getEnvInt("EMBEDDING_DIM", 768),

		ZillizURI:      getEnv("ZILLIZ_URI", ""),
		ZillizToken:    getEnv("ZILLIZ_TOKEN", ""),
		CollectionName: collectionName,

		ScanExtensions: getEnvSlice("SCAN_EXTENSIONS", []string{".go", ".vue", ".js", ".ts", ".py", ".md"}),
		ChunkSize:      getEnvInt("CHUNK_SIZE", 800),
		MaxChunkSize:   getEnvInt("MAX_CHUNK_SIZE", 1500),
		AutoIndex:      getEnvBool("AUTO_INDEX", true),
		ProjectPath:    getEnv("PROJECT_PATH", ""),
		IndexStatePath: getEnv("INDEX_STATE_PATH", ""),
	}
}

// normalizeCollectionName 规范化 Milvus 集合名称
// Milvus 集合名称规则：以字母或下划线开头，只允许字母、数字、下划线，长度 1~255
func normalizeCollectionName(name string) string {
	original := name

	// 将连字符和其他非法字符替换为下划线
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	name = re.ReplaceAllString(name, "_")

	// 确保不以数字开头（如果是，添加前缀 "c_"）
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "c_" + name
	}

	// 确保不为空
	if name == "" {
		name = "code_context"
	}

	// 截断至 255 字符
	if len(name) > 255 {
		name = name[:255]
	}

	if name != original {
		fmt.Fprintf(os.Stderr, "警告: 集合名称已规范化: %q → %q (Milvus 集合名称只允许字母、数字、下划线)\n", original, name)
	}

	return name
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
