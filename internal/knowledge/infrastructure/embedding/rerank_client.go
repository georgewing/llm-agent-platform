package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"llm-agent-platform/internal/knowledge/domain"
)

type RerankClient struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
	model      string
}

// RerankRequest 重排请求体
type RerankRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	TopN            int      `json:"top_n,omitempty"`
	ReturnDocs      bool     `json:"return_documents,omitempty"` // 是否返回文档内容
	MaxChunksPerDoc int      `json:"max_chunks_per_doc,omitempty"`
}

// RerankResult 单个重排结果
type RerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
	Document       string  `json:"document,omitempty"` // 可选返回
}

// RerankResponse 重排响应体
type RerankResponse struct {
	Results []RerankResult `json:"results"`
	Usage   struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func getEnvWithDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func NewRerankClient() *RerankClient {
	baseURL := os.Getenv("RERANK_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.siliconflow.cn/v1" // 默认硅基流动
	}
	return &RerankClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiKey:     os.Getenv("RERANK_API_KEY"),
		baseURL:    baseURL,
		model:      getEnvWithDefault("RERANK_MODEL", "BAAI/bge-reranker-v2-m3"),
	}
}

// 实现领域层接口
var _ domain.RerankService = (*RerankClient)(nil)

// Rerank 执行重排
func (c *RerankClient) Rerank(ctx context.Context, query string, documents []string, topN int) ([]float64, error) {
	if len(documents) == 0 {
		return nil, fmt.Errorf("documents cannot be empty")
	}
	if c.apiKey == "" {
		return nil, fmt.Errorf("RERANK_API_KEY not set")
	}

	reqBody := RerankRequest{
		Query:     query,
		Documents: documents,
		TopN:      topN,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal rerank request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/rerank", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rerank http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rerank API error: status=%d", resp.StatusCode)
	}

	var apiResp RerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode rerank response failed: %w", err)
	}

	// 将结果映射回原始文档顺序的分数数组
	scores := make([]float64, len(documents))
	for _, r := range apiResp.Results {
		if r.Index >= 0 && r.Index < len(scores) {
			scores[r.Index] = r.RelevanceScore
		}
	}
	return scores, nil
}
