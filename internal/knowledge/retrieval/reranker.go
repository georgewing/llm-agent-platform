package retrieval

import (
	"context"
	"sort"

	"llm-agent-platform/internal/knowledge/domain"
)

// Reranker 重排器接口
type Reranker interface {
	Rerank(ctx context.Context, query string, chunks []*domain.Chunk, topK int) ([]*domain.Chunk, error)
}

// CrossEncoderReranker 调用外部 CrossEncoder 模型 API (如 BGE-Reranker, Cohere Rerank)
type CrossEncoderReranker struct {
	apiClient RerankAPIClient // 假设在 internal/embedding 封装了相关HTTP客户端
}

// Rerank 实现
func (r *CrossEncoderReranker) Rerank(ctx context.Context, query string, chunks []*domain.Chunk, topK int) ([]*domain.Chunk, error) {
	if len(chunks) == 0 {
		return chunks, nil
	}

	// 提取文本列表发给 Rerank 模型
	var texts []string
	for _, c := range chunks {
		texts = append(texts, c.Content)
	}

	// 调用远端 CrossEncoder 进行重新打分
	// 期望返回的 scores 顺序与 texts 顺序一致，取值一般是不限范围的 Logit 或 0~1 的 sigmoid 值
	scores, err := r.apiClient.CrossEncoderScore(ctx, query, texts)
	if err != nil {
		return nil, err
	}

	// 更新 Chunk 的分数
	for i, c := range chunks {
		c.Score = scores[i]
	}

	// 按 Rerank 后的 Score 重新降序排列
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Score > chunks[j].Score
	})

	// 截断 Top K
	if len(chunks) > topK {
		return chunks[:topK], nil
	}
	return chunks, nil
}
