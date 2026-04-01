package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"enterprise-agent/internal/knowledge/domain"
	"github.com/elastic/go-elasticsearch/v8"
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
	}

	// 2. 发送请求到 ES
	// 3. 解析 Response 中的 Hits
	// 4. 将 ES 的 _id 和 _score 映射到 domain.Chunk
	return []*domain.Chunk{}, nil
}
