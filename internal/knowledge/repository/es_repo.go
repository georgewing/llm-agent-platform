package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"llm-agent-platform/internal/knowledge/domain"
)

type ESRepo struct {
	client    *elasticsearch.Client
	indexName string
}

func NewESRepo(c *elasticsearch.Client, indexName string) *ESRepo {
	return &ESRepo{client: c, indexName: indexName}
}

// SearchKeyword 实现基于 BM25 的全文检索
func (r *ESRepo) SearchKeyword(ctx context.Context, query string, topK int) ([]*domain.Chunk, error) {
	// 1. 构建 ES 的 DSL 查询语句 (match 结合 fuzziness)
	queryBody := map[string]interface{}{
		"query": map[string]interface{}{
			"match": map[string]interface{}{
				"content": map[string]interface{}{
					"query":     query,
					"fuzziness": "AUTO", // 开启模糊匹配，容错错别字
				},
			},
		},
		"size": topK,
		// 可选：加 min_score 过滤低质量结果
		// "min_score": 0.5,
	}

	// 2. 发送请求到 ES
	res, err := r.client.Search(
		r.client.Search.WithContext(ctx),
		r.client.Search.WithIndex(r.indexName),
		r.client.Search.WithBody(bytes.NewReader(bodyBytes)),
		r.client.Search.WithTrackTotalHits(false),
		r.client.Search.WithPretty(),
	)
	if err != nil {
		return nil, fmt.Errorf("es search request failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("es search error: %s", res.String())
	}

	// 3. 解析 Response 中的 Hits
	type hit struct {
		ID     string  `json:"_id"`
		Score  float64 `json:"_score"`
		Source struct {
			Content string `json:"content"`
		} `json:"_source"`
	}

	type esResponse struct {
		Hits struct {
			Hits []hit `json:"hits"`
		} `json:"hits"`
	}

	var resp esResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode es response failed: %w", err)
	}

	// 4. 将 ES 的 _id 和 _score 映射到 domain.Chunk
	chunks := make([]*domain.Chunk, 0, len(resp.Hits.Hits))
	for _, h := range resp.Hits.Hits {
		chunks = append(chunks, &domain.Chunk{
			ID:      h.ID,
			Score:   h.Score,          // BM25 分数（越高越好）
			Content: h.Source.Content, // 直接带回内容，避免额外 PG 查询
		})
	}

	return chunks, nil
}

func (r *ESRepo) IndexKeywords(ctx context.Context, chunks []*domain.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, chunk := range chunks {
		// Bulk meta line
		meta := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": r.indexName,
				"_id":    chunk.ID,
			},
		}
		if err := json.NewEncoder(&buf).Encode(meta); err != nil {
			return fmt.Errorf("encode meta failed: %w", err)
		}
		buf.WriteByte('\n')

		// Document body (content is the only field needed for match query)
		doc := map[string]interface{}{
			"content":     chunk.Content,
			"document_id": chunk.DocumentID, // useful for future filtering
		}
		if err := json.NewEncoder(&buf).Encode(doc); err != nil {
			return fmt.Errorf("encode doc failed: %w", err)
		}
		buf.WriteByte('\n')
	}

	res, err := r.client.Bulk(
		bytes.NewReader(buf.Bytes()),
		r.client.Bulk.WithContext(ctx),
		r.client.Bulk.WithRefresh("wait_for"), // immediate visibility for small batches
	)
	if err != nil {
		return fmt.Errorf("es bulk request failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("es bulk error: %s", res.String())
	}

	return nil
}
