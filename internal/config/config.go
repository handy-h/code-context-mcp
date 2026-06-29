package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var collectionNameRe = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// EmbeddingProviderType 嵌入模型提供商类型
type EmbeddingProviderType string

const (
	ProviderOllama EmbeddingProviderType = "ollama"
	ProviderOpenAI EmbeddingProviderType = "openai"
	ProviderGemini EmbeddingProviderType = "gemini"
)

// VectorStoreType controls where vectors are persisted.
type VectorStoreType string

const (
	VectorStoreLocalJSONL VectorStoreType = "local-jsonl"
	VectorStoreZilliz     VectorStoreType = "zilliz"
)

// Config MCP 服务器全局配置
type Config struct {
	// 嵌入模型配置
	EmbeddingProvider EmbeddingProviderType
	EmbeddingDim      int

	// Ollama 配置
	OllamaURL   string
	OllamaModel string

	// OpenAI 兼容 API 配置
	OpenAIBaseURL string
	OpenAIModel   string
	OpenAIAPIKey  string

	// Google Gemini 配置
	GeminiBaseURL string
	GeminiModel   string
	GeminiAPIKey  string

	// Vector store configuration
	VectorStore     VectorStoreType
	VectorStorePath string
	ZillizURI       string
	ZillizToken     string
	CollectionName  string

	// 索引配置
	ScanExtensions []string
	ChunkSize      int
	MaxChunkSize   int
	AutoIndex      bool
	ProjectPath    string
	IndexStatePath string

	// 超时配置
	SearchTimeout time.Duration
	IndexTimeout  time.Duration

	// Token 统计配置
	TokenStatsEnabled                bool
	TokenStatsPath                   string
	TokenStatsCharsPerToken          float64
	TokenStatsCodeSearchBaseline     int
	TokenStatsFileContextBaseline    int
	TokenStatsSymbolSearchBaseline   int
	TokenStatsImpactAnalysisBaseline int
	TokenStatsRetentionDays          int
}

// LoadConfig 从环境变量加载配置，提供合理默认值
func LoadConfig() Config {
	collectionName := normalizeCollectionName(getEnv("COLLECTION_NAME", "code_context"))
	vectorStore := parseVectorStore(getEnv("VECTOR_STORE", "local"))

	provider := getEnv("EMBEDDING_PROVIDER", "ollama")
	var embeddingProvider EmbeddingProviderType
	switch provider {
	case "openai":
		embeddingProvider = ProviderOpenAI
	case "gemini":
		embeddingProvider = ProviderGemini
	default:
		embeddingProvider = ProviderOllama
	}

	return Config{
		EmbeddingProvider: embeddingProvider,
		EmbeddingDim:      getEnvInt("EMBEDDING_DIM", 768),

		OllamaURL:     getEnv("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:   getEnv("OLLAMA_EMBED_MODEL", "nomic-embed-text:latest"),
		OpenAIBaseURL: getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIModel:   getEnv("OPENAI_EMBED_MODEL", "text-embedding-ada-002"),
		OpenAIAPIKey:  getEnv("OPENAI_API_KEY", ""),
		GeminiBaseURL: getEnv("GEMINI_BASE_URL", "https://generativelanguage.googleapis.com/v1beta"),
		GeminiModel:   getEnv("GEMINI_EMBED_MODEL", "embedding-001"),
		GeminiAPIKey:  getEnv("GEMINI_API_KEY", ""),

		VectorStore:     vectorStore,
		VectorStorePath: getEnv("VECTOR_STORE_PATH", defaultLocalVectorStorePath(collectionName)),
		ZillizURI:       getEnv("ZILLIZ_URI", ""),
		ZillizToken:     getEnv("ZILLIZ_TOKEN", ""),
		CollectionName:  collectionName,

		ScanExtensions: getEnvSlice("SCAN_EXTENSIONS", []string{".go", ".vue", ".js", ".ts", ".py", ".md", ".rs"}),
		ChunkSize:      getEnvInt("CHUNK_SIZE", 800),
		MaxChunkSize:   getEnvInt("MAX_CHUNK_SIZE", 1500),
		AutoIndex:      getEnvBool("AUTO_INDEX", true),
		ProjectPath:    getEnv("PROJECT_PATH", ""),
		IndexStatePath: getEnv("INDEX_STATE_PATH", ""),

		SearchTimeout: time.Duration(getEnvInt("SEARCH_TIMEOUT_SECONDS", 30)) * time.Second,
		IndexTimeout:  time.Duration(getEnvInt("INDEX_TIMEOUT_SECONDS", 300)) * time.Second,

		TokenStatsEnabled:                getEnvBool("TOKEN_STATS_ENABLED", false),
		TokenStatsPath:                   getEnv("TOKEN_STATS_PATH", defaultTokenStatsPath()),
		TokenStatsCharsPerToken:          getEnvFloat64("TOKEN_STATS_CHARS_PER_TOKEN", 4.0),
		TokenStatsCodeSearchBaseline:     getEnvInt("TOKEN_STATS_CODE_SEARCH_BASELINE", 2000),
		TokenStatsFileContextBaseline:    getEnvInt("TOKEN_STATS_FILE_CONTEXT_BASELINE", 3000),
		TokenStatsSymbolSearchBaseline:   getEnvInt("TOKEN_STATS_SYMBOL_SEARCH_BASELINE", 8000),
		TokenStatsImpactAnalysisBaseline: getEnvInt("TOKEN_STATS_IMPACT_ANALYSIS_BASELINE", 12000),
		TokenStatsRetentionDays:          getEnvInt("TOKEN_STATS_RETENTION_DAYS", 90),
	}
}

// ValidateEmbeddingDim 根据 provider 和 model 验证/建议 embedding 维度
// 返回 expected dim (0 表示未知), 以及警告消息
func (c Config) ValidateEmbeddingDim() (int, string) {
	switch c.EmbeddingProvider {
	case ProviderOllama:
		if c.OllamaModel == "nomic-embed-text" || strings.Contains(c.OllamaModel, "nomic-embed-text") {
			if c.EmbeddingDim != 768 {
				return 768, fmt.Sprintf("nomic-embed-text 模型输出 768 维向量，当前配置 EMBEDDING_DIM=%d，建议修改为 768", c.EmbeddingDim)
			}
		}
	case ProviderOpenAI:
		knownDims := map[string]int{
			"text-embedding-ada-002":      1536,
			"text-embedding-3-small":     1536,
			"text-embedding-3-large":     3072,
		}
		if expected, ok := knownDims[c.OpenAIModel]; ok {
			if c.EmbeddingDim != expected {
				return expected, fmt.Sprintf("%s 模型输出 %d 维向量，当前配置 EMBEDDING_DIM=%d，建议修改", c.OpenAIModel, expected, c.EmbeddingDim)
			}
		}
	case ProviderGemini:
		if strings.Contains(c.GeminiModel, "embedding-001") {
			if c.EmbeddingDim != 768 {
				return 768, fmt.Sprintf("embedding-001 模型输出 768 维向量，当前配置 EMBEDDING_DIM=%d，建议修改为 768", c.EmbeddingDim)
			}
		}
	}
	return 0, ""
}

func parseVectorStore(value string) VectorStoreType {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "zilliz", "milvus":
		return VectorStoreZilliz
	case "local", "jsonl", "local-jsonl", "":
		return VectorStoreLocalJSONL
	default:
		return VectorStoreLocalJSONL
	}
}

func defaultLocalVectorStorePath(collectionName string) string {
	dir := "."
	if exe, err := os.Executable(); err == nil {
		dir = filepath.Dir(exe)
	}
	return filepath.Join(dir, collectionName+".jsonl")
}

func defaultTokenStatsPath() string {
	dir := "."
	if exe, err := os.Executable(); err == nil {
		dir = filepath.Dir(exe)
	}
	return filepath.Join(dir, "token-stats.json")
}

// normalizeCollectionName 规范化 Milvus 集合名称
// Milvus 集合名称规则：以字母或下划线开头，只允许字母、数字、下划线，长度 1~255
func normalizeCollectionName(name string) string {
	original := name

	// 将连字符和其他非法字符替换为下划线
	name = collectionNameRe.ReplaceAllString(name, "_")

	// 确保不以数字开头（如果是，添加前缀 "c_"）
	if name != "" && name[0] >= '0' && name[0] <= '9' {
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
		fmt.Fprintf(os.Stderr, "警告: 环境变量 %s 的值 %q 无法解析为整数，使用默认值 %v\n", key, v, fallback)
		return fallback
	}
	return n
}

func getEnvFloat64(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告: 环境变量 %s 的值 %q 无法解析为浮点数，使用默认值 %v\n", key, v, fallback)
		return fallback
	}
	return f
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
		fmt.Fprintf(os.Stderr, "警告: 环境变量 %s 的值 %q 无效，使用默认值 %v\n", key, v, fallback)
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
