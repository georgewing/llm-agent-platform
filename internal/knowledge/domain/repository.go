package domain

import "context"

// RerankService 领域层定义的 Rerank 服务接口
type RerankService interface {
	Rerank(ctx context.Context, query string, texts []string, topN int) ([]float64, error)
}

// EmbeddingService 领域层定义的 Embedding 服务接口
type EmbeddingService interface {
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// VectorCalculator 领域层定义的向量计算接口
type VectorCalculator interface {
	CosineSimilarity(v1, v2 []float32) (float32, error)
}
