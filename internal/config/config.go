package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// EmbeddingProviderType 宓屽叆妯″瀷鎻愪緵鍟嗙被鍨?
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

// Config MCP 鏈嶅姟鍣ㄥ叏灞€閰嶇疆
type Config struct {
	// 宓屽叆妯″瀷閰嶇疆
	EmbeddingProvider EmbeddingProviderType
	EmbeddingDim      int

	// Ollama 閰嶇疆
	OllamaURL   string
	OllamaModel string

	// OpenAI 鍏煎 API 閰嶇疆
	OpenAIBaseURL string
	OpenAIModel   string
	OpenAIAPIKey  string

	// Google Gemini 閰嶇疆
	GeminiBaseURL string
	GeminiModel   string
	GeminiAPIKey  string

	// Vector store configuration
	VectorStore     VectorStoreType
	VectorStorePath string
	ZillizURI       string
	ZillizToken     string
	CollectionName  string

	// 绱㈠紩閰嶇疆
	ScanExtensions []string
	ChunkSize      int
	MaxChunkSize   int
	AutoIndex      bool
	ProjectPath    string
	IndexStatePath string
}

// LoadConfig 浠庣幆澧冨彉閲忓姞杞介厤缃紝鎻愪緵鍚堢悊榛樿鍊?
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
	case "ollama":
		fallthrough
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

		ScanExtensions: getEnvSlice("SCAN_EXTENSIONS", []string{".go", ".vue", ".js", ".ts", ".py", ".md"}),
		ChunkSize:      getEnvInt("CHUNK_SIZE", 800),
		MaxChunkSize:   getEnvInt("MAX_CHUNK_SIZE", 1500),
		AutoIndex:      getEnvBool("AUTO_INDEX", true),
		ProjectPath:    getEnv("PROJECT_PATH", ""),
		IndexStatePath: getEnv("INDEX_STATE_PATH", ""),
	}
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
	if filepath.Base(dir) != "code-text" {
		dir = filepath.Join(dir, "code-text")
	}
	return filepath.Join(dir, collectionName+".jsonl")
}

// normalizeCollectionName 瑙勮寖鍖?Milvus 闆嗗悎鍚嶇О
// Milvus 闆嗗悎鍚嶇О瑙勫垯锛氫互瀛楁瘝鎴栦笅鍒掔嚎寮€澶达紝鍙厑璁稿瓧姣嶃€佹暟瀛椼€佷笅鍒掔嚎锛岄暱搴?1~255
func normalizeCollectionName(name string) string {
	original := name

	// 灏嗚繛瀛楃鍜屽叾浠栭潪娉曞瓧绗︽浛鎹负涓嬪垝绾?
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	name = re.ReplaceAllString(name, "_")

	// 纭繚涓嶄互鏁板瓧寮€澶达紙濡傛灉鏄紝娣诲姞鍓嶇紑 "c_"锛?
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "c_" + name
	}

	// 纭繚涓嶄负绌?
	if name == "" {
		name = "code_context"
	}

	// 鎴柇鑷?255 瀛楃
	if len(name) > 255 {
		name = name[:255]
	}

	if name != original {
		fmt.Fprintf(os.Stderr, "璀﹀憡: 闆嗗悎鍚嶇О宸茶鑼冨寲: %q 鈫?%q (Milvus 闆嗗悎鍚嶇О鍙厑璁稿瓧姣嶃€佹暟瀛椼€佷笅鍒掔嚎)\n", original, name)
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
