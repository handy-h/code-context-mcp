package config

import (
	"os"
	"testing"
)

func TestParseVectorStore(t *testing.T) {
	tests := []struct {
		input string
		want  VectorStoreType
	}{
		{"local", VectorStoreLocalJSONL},
		{"jsonl", VectorStoreLocalJSONL},
		{"local-jsonl", VectorStoreLocalJSONL},
		{"", VectorStoreLocalJSONL},
		{"zilliz", VectorStoreZilliz},
		{"milvus", VectorStoreZilliz},
		{"ZILLIZ", VectorStoreZilliz},
		{"unknown", VectorStoreLocalJSONL},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseVectorStore(tt.input); got != tt.want {
				t.Errorf("parseVectorStore(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeCollectionName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid", "code_context", "code_context"},
		{"with_hyphen", "code-context", "code_context"},
		{"with_dot", "code.context", "code_context"},
		{"starts_with_digit", "123abc", "c_123abc"},
		{"empty", "", "code_context"},
		{"valid_upper", "MyCollection", "MyCollection"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeCollectionName(tt.input); got != tt.want {
				t.Errorf("normalizeCollectionName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear all relevant env vars
	envVars := []string{
		"EMBEDDING_PROVIDER", "EMBEDDING_DIM",
		"OLLAMA_URL", "OLLAMA_EMBED_MODEL",
		"OPENAI_BASE_URL", "OPENAI_EMBED_MODEL", "OPENAI_API_KEY",
		"GEMINI_BASE_URL", "GEMINI_EMBED_MODEL", "GEMINI_API_KEY",
		"VECTOR_STORE", "VECTOR_STORE_PATH",
		"ZILLIZ_URI", "ZILLIZ_TOKEN", "COLLECTION_NAME",
		"SCAN_EXTENSIONS", "CHUNK_SIZE", "MAX_CHUNK_SIZE",
		"AUTO_INDEX", "PROJECT_PATH", "INDEX_STATE_PATH",
		"SEARCH_TIMEOUT_SECONDS", "INDEX_TIMEOUT_SECONDS",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}

	cfg := LoadConfig()

	if cfg.EmbeddingProvider != ProviderOllama {
		t.Errorf("default EmbeddingProvider = %q, want ollama", cfg.EmbeddingProvider)
	}
	if cfg.EmbeddingDim != 768 {
		t.Errorf("default EmbeddingDim = %d, want 768", cfg.EmbeddingDim)
	}
	if cfg.OllamaURL != "http://localhost:11434" {
		t.Errorf("default OllamaURL = %q", cfg.OllamaURL)
	}
	if cfg.OllamaModel != "nomic-embed-text:latest" {
		t.Errorf("default OllamaModel = %q", cfg.OllamaModel)
	}
	if cfg.VectorStore != VectorStoreLocalJSONL {
		t.Errorf("default VectorStore = %q, want local-jsonl", cfg.VectorStore)
	}
	if cfg.CollectionName != "code_context" {
		t.Errorf("default CollectionName = %q", cfg.CollectionName)
	}
	if cfg.AutoIndex != true {
		t.Errorf("default AutoIndex = %v, want true", cfg.AutoIndex)
	}
	if len(cfg.ScanExtensions) == 0 {
		t.Error("default ScanExtensions should not be empty")
	}
	if cfg.ChunkSize != 800 {
		t.Errorf("default ChunkSize = %d, want 800", cfg.ChunkSize)
	}
	if cfg.MaxChunkSize != 1500 {
		t.Errorf("default MaxChunkSize = %d, want 1500", cfg.MaxChunkSize)
	}
}

func TestLoadConfig_CustomValues(t *testing.T) {
	t.Setenv("EMBEDDING_PROVIDER", "openai")
	t.Setenv("EMBEDDING_DIM", "1536")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("VECTOR_STORE", "zilliz")
	t.Setenv("ZILLIZ_URI", "https://test.zilliz.com")
	t.Setenv("ZILLIZ_TOKEN", "test-token")
	t.Setenv("COLLECTION_NAME", "my_collection")
	t.Setenv("CHUNK_SIZE", "1000")
	t.Setenv("AUTO_INDEX", "false")

	cfg := LoadConfig()

	if cfg.EmbeddingProvider != ProviderOpenAI {
		t.Errorf("EmbeddingProvider = %q, want openai", cfg.EmbeddingProvider)
	}
	if cfg.EmbeddingDim != 1536 {
		t.Errorf("EmbeddingDim = %d, want 1536", cfg.EmbeddingDim)
	}
	if cfg.OpenAIAPIKey != "test-key" {
		t.Errorf("OpenAIAPIKey = %q, want test-key", cfg.OpenAIAPIKey)
	}
	if cfg.VectorStore != VectorStoreZilliz {
		t.Errorf("VectorStore = %q, want zilliz", cfg.VectorStore)
	}
	if cfg.ZillizURI != "https://test.zilliz.com" {
		t.Errorf("ZillizURI = %q", cfg.ZillizURI)
	}
	if cfg.ZillizToken != "test-token" {
		t.Errorf("ZillizToken = %q", cfg.ZillizToken)
	}
	if cfg.CollectionName != "my_collection" {
		t.Errorf("CollectionName = %q, want my_collection", cfg.CollectionName)
	}
	if cfg.ChunkSize != 1000 {
		t.Errorf("ChunkSize = %d, want 1000", cfg.ChunkSize)
	}
	if cfg.AutoIndex != false {
		t.Errorf("AutoIndex = %v, want false", cfg.AutoIndex)
	}
}

func TestLoadConfig_EmbeddingProviderSwitch(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     EmbeddingProviderType
	}{
		{"ollama", "ollama", ProviderOllama},
		{"openai", "openai", ProviderOpenAI},
		{"gemini", "gemini", ProviderGemini},
		{"empty", "", ProviderOllama},
		{"unknown", "unknown", ProviderOllama},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("EMBEDDING_PROVIDER", tt.provider)
			cfg := LoadConfig()
			if cfg.EmbeddingProvider != tt.want {
				t.Errorf("EmbeddingProvider = %q, want %q", cfg.EmbeddingProvider, tt.want)
			}
		})
	}
}
