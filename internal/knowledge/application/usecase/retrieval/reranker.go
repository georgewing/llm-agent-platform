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
	service domain.RerankService // 依赖接口
}

func NewCrossEncoderReranker(service domain.RerankService) *CrossEncoderReranker {
	return &CrossEncoderReranker{service: service}
}

// Rerank 实现
func (r *CrossEncoderReranker) Rerank(ctx context.Context, query string, chunks []*domain.Chunk, topK int) ([]*domain.Chunk, error) {
	if len(chunks) == 0 {
		return chunks, nil
	}

	// 提取文本列表发给 Rerank 模型
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Content
	}

	// 调用远端 CrossEncoder 进行重新打分
	scores, err := r.service.Rerank(ctx, query, texts, topK)
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
