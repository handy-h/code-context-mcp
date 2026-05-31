package embedding

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/handy-h/code-context-mcp/internal/config"
)

func TestNewEmbeddingProvider_Ollama(t *testing.T) {
	cfg := config.Config{
		EmbeddingProvider: config.ProviderOllama,
		OllamaURL:         "http://localhost:11434",
		OllamaModel:       "nomic-embed-text:latest",
		EmbeddingDim:      768,
	}
	provider, err := NewEmbeddingProvider(cfg)
	if err != nil {
		t.Fatalf("NewEmbeddingProvider(ollama) error: %v", err)
	}
	if _, ok := provider.(*OllamaProvider); !ok {
		t.Errorf("returned %T, want *OllamaProvider", provider)
	}
	if provider.GetDimension() != 768 {
		t.Errorf("GetDimension() = %d, want 768", provider.GetDimension())
	}
}

func TestNewEmbeddingProvider_OpenAI(t *testing.T) {
	cfg := config.Config{
		EmbeddingProvider: config.ProviderOpenAI,
		OpenAIBaseURL:     "https://api.openai.com/v1",
		OpenAIModel:       "text-embedding-ada-002",
		OpenAIAPIKey:      "test-key",
		EmbeddingDim:      1536,
	}
	provider, err := NewEmbeddingProvider(cfg)
	if err != nil {
		t.Fatalf("NewEmbeddingProvider(openai) error: %v", err)
	}
	if _, ok := provider.(*OpenAIProvider); !ok {
		t.Errorf("returned %T, want *OpenAIProvider", provider)
	}
	if provider.GetDimension() != 1536 {
		t.Errorf("GetDimension() = %d, want 1536", provider.GetDimension())
	}
}

func TestNewEmbeddingProvider_Gemini(t *testing.T) {
	cfg := config.Config{
		EmbeddingProvider: config.ProviderGemini,
		GeminiBaseURL:     "https://generativelanguage.googleapis.com/v1beta",
		GeminiModel:       "embedding-001",
		GeminiAPIKey:      "test-key",
		EmbeddingDim:      768,
	}
	provider, err := NewEmbeddingProvider(cfg)
	if err != nil {
		t.Fatalf("NewEmbeddingProvider(gemini) error: %v", err)
	}
	if _, ok := provider.(*GeminiProvider); !ok {
		t.Errorf("returned %T, want *GeminiProvider", provider)
	}
}

func TestNewEmbeddingProvider_Unsupported(t *testing.T) {
	cfg := config.Config{
		EmbeddingProvider: "unsupported",
	}
	_, err := NewEmbeddingProvider(cfg)
	if err == nil {
		t.Error("NewEmbeddingProvider(unsupported) should return error")
	}
}

func TestOllamaProvider_GetEmbedding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		resp := ollamaResponse{
			Embedding: []float32{0.1, 0.2, 0.3},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL, "test-model", 3)
	vec, err := provider.GetEmbedding("hello world")
	if err != nil {
		t.Fatalf("GetEmbedding error: %v", err)
	}
	if len(vec) != 3 {
		t.Errorf("embedding length = %d, want 3", len(vec))
	}
}

func TestOllamaProvider_GetEmbedding_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL, "test-model", 3)
	_, err := provider.GetEmbedding("hello")
	if err == nil {
		t.Error("GetEmbedding(500) should return error")
	}
}

func TestOllamaProvider_GetEmbedding_EmptyVector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaResponse{Embedding: []float32{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL, "test-model", 3)
	_, err := provider.GetEmbedding("hello")
	if err == nil {
		t.Error("GetEmbedding(empty vector) should return error")
	}
}

func TestOllamaProvider_GetDimension(t *testing.T) {
	provider := NewOllamaProvider("http://localhost", "model", 768)
	if provider.GetDimension() != 768 {
		t.Errorf("GetDimension() = %d, want 768", provider.GetDimension())
	}
}

func TestOllamaProvider_TrailingSlash(t *testing.T) {
	provider := NewOllamaProvider("http://localhost:11434/", "model", 768)
	if provider.url != "http://localhost:11434" {
		t.Errorf("url = %q, want trailing slash removed", provider.url)
	}
}

func TestOpenAIProvider_GetEmbedding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", auth)
		}
		resp := openaiEmbeddingResponse{
			Data: []struct {
				Object    string    `json:"object"`
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Embedding: []float32{0.1, 0.2, 0.3}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOpenAIProvider(server.URL, "test-model", "test-key", 3)
	vec, err := provider.GetEmbedding("hello world")
	if err != nil {
		t.Fatalf("GetEmbedding error: %v", err)
	}
	if len(vec) != 3 {
		t.Errorf("embedding length = %d, want 3", len(vec))
	}
}

func TestOpenAIProvider_GetEmbedding_NoAPIKey(t *testing.T) {
	provider := NewOpenAIProvider("http://localhost", "model", "", 3)
	_, err := provider.GetEmbedding("hello")
	if err == nil {
		t.Error("GetEmbedding(no API key) should return error")
	}
}

func TestOpenAIProvider_GetEmbedding_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid key"}`))
	}))
	defer server.Close()

	provider := NewOpenAIProvider(server.URL, "model", "bad-key", 3)
	_, err := provider.GetEmbedding("hello")
	if err == nil {
		t.Error("GetEmbedding(401) should return error")
	}
}

func TestOpenAIProvider_GetDimension(t *testing.T) {
	provider := NewOpenAIProvider("http://localhost", "model", "key", 1536)
	if provider.GetDimension() != 1536 {
		t.Errorf("GetDimension() = %d, want 1536", provider.GetDimension())
	}
}

func TestGeminiProvider_GetEmbedding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"embedding": map[string]interface{}{
				"values": []float64{0.1, 0.2, 0.3},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewGeminiProvider(server.URL, "embedding-001", "test-key", 3)
	vec, err := provider.GetEmbedding("hello world")
	if err != nil {
		t.Fatalf("GetEmbedding error: %v", err)
	}
	if len(vec) != 3 {
		t.Errorf("embedding length = %d, want 3", len(vec))
	}
}

func TestGeminiProvider_GetEmbedding_NoAPIKey(t *testing.T) {
	provider := NewGeminiProvider("http://localhost", "model", "", 3)
	_, err := provider.GetEmbedding("hello")
	if err == nil {
		t.Error("GetEmbedding(no API key) should return error")
	}
}

func TestGeminiProvider_GetEmbedding_DimensionMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"embedding": map[string]interface{}{
				"values": []float64{0.1, 0.2, 0.3, 0.4, 0.5},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewGeminiProvider(server.URL, "model", "key", 3)
	_, err := provider.GetEmbedding("hello")
	if err == nil {
		t.Error("GetEmbedding(dimension mismatch) should return error")
	}
}

func TestGeminiProvider_GetEmbedding_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "bad request"}}`))
	}))
	defer server.Close()

	provider := NewGeminiProvider(server.URL, "model", "key", 3)
	_, err := provider.GetEmbedding("hello")
	if err == nil {
		t.Error("GetEmbedding(400) should return error")
	}
}

func TestGeminiProvider_GetDimension(t *testing.T) {
	provider := NewGeminiProvider("http://localhost", "model", "key", 768)
	if provider.GetDimension() != 768 {
		t.Errorf("GetDimension() = %d, want 768", provider.GetDimension())
	}
}
