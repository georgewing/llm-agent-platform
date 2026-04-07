package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"llm-agent-platform/internal/knowledge/repository"
)

type EmbeddingClient struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
	model      string
}

func NewEmbeddingClient() repository.EmbeddingService {
	baseURL := os.Getenv("EMBEDDING_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1" // 支持任意 OpenAI 兼容接口（Qwen、DeepSeek、Ollama、Azure 等）
	}
	return &EmbeddingClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiKey:     os.Getenv("EMBEDDING_API_KEY"),
		baseURL:    baseURL,
		model:      getEnvWithDefault("EMBEDDING_MODEL", "text-embedding-3-small"),
	}
}

func getEnvWithDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (c *EmbeddingClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if c.apiKey == "" {
		return nil, fmt.Errorf("EMBEDDING_API_KEY not set")
	}

	reqBody := map[string]interface{}{
		"model": c.model,
		"input": texts,
		// 可选：dimensions: 1536（节约 token）
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API error: %d", resp.StatusCode)
	}

	type apiResp struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	var apiResponse apiResp
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("decode embedding response failed: %w", err)
	}

	result := make([][]float32, len(apiResponse.Data))
	for i, item := range apiResponse.Data {
		vec := make([]float32, len(item.Embedding))
		for j, v := range item.Embedding {
			vec[j] = float32(v)
		}
		result[i] = vec
	}
	return result, nil
}
