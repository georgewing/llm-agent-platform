package retrieval

import (
	"context"
	"math"
	"sort"
	"sync"

	"llm-agent-platform/internal/knowledge/domain"
	"llm-agent-platform/internal/knowledge/repository"
)

// HybridRetriever 混合检索器
type HybridRetriever struct {
	vectorRepo  repository.VectorRepo
	keywordRepo repository.KeywordRepo
	reranker    Reranker // 可选的重排器组件
	alpha       float64  // 向量权重
	beta        float64  // 关键词权重
}

// NewHybridRetriever 初始化混合检索器
func NewHybridRetriever(vr repository.VectorRepo, kr repository.KeywordRepo, reranker Reranker, alpha, beta float64) *HybridRetriever {
	return &HybridRetriever{
		vectorRepo:  vr,
		keywordRepo: kr,
		reranker:    reranker,
		alpha:       alpha,
		beta:        beta,
	}
}

// Retrieve 执行完整的检索与重排Pipeline
func (h *HybridRetriever) Retrieve(ctx context.Context, query string, queryVector []float32, topK int) ([]*domain.Chunk, error) {
	var (
		wg        sync.WaitGroup
		vecChunks []*domain.Chunk
		kwChunks  []*domain.Chunk
		vecErr    error
		kwErr     error
	)

	wg.Add(2)

	// 1. 并发调用 Milvus 进行 AnnSearch 向量召回
	go func() {
		defer wg.Done()
		vecChunks, vecErr = h.vectorRepo.SearchVector(ctx, queryVector, topK)
	}()

	// 2. 并发调用 ES 进行 BM25 + Fuzzy 关键词召回
	go func() {
		defer wg.Done()
		kwChunks, kwErr = h.keywordRepo.SearchKeyword(ctx, query, topK)
	}()

	wg.Wait()

	// 容错处理：如果两路都失败才返回错误，否则部分降级
	if vecErr != nil && kwErr != nil {
		return nil, vecErr
	}

	// 3. 分数归一化 (Min-Max Normalization)
	h.normalizeScores(vecChunks)
	h.normalizeScores(kwChunks)

	// 4. 线性加权融合 (Linear Weighting Fusion)
	fusedChunks := h.linearWeightFusion(vecChunks, kwChunks, topK*2) // 召回阶段多保留一些给重排

	// 5. 可选：重排序阶段 (CrossEncoder / LLM Rerank)
	if h.reranker != nil && len(fusedChunks) > 0 {
		rerankedChunks, err := h.reranker.Rerank(ctx, query, fusedChunks, topK)
		if err == nil {
			return rerankedChunks, nil
		}
		// 如果 Rerank 失败（例如调大模型超时），优雅降级，直接使用召回结果
	}

	// 截取 Top K
	if len(fusedChunks) > topK {
		fusedChunks = fusedChunks[:topK]
	}

	return fusedChunks, nil
}

// normalizeScores Min-Max 归一化，将得分映射到 0~1 区间
func (h *HybridRetriever) normalizeScores(chunks []*domain.Chunk) {
	if len(chunks) == 0 {
		return
	}
	if len(chunks) == 1 {
		chunks[0].Score = 1.0
		return
	}

	minScore := chunks[0].Score
	maxScore := chunks[0].Score

	for _, c := range chunks {
		if c.Score < minScore {
			minScore = c.Score
		}
		if c.Score > maxScore {
			maxScore = c.Score
		}
	}

	diff := maxScore - minScore
	if diff == 0 { // 避免除以0
		for _, c := range chunks {
			c.Score = 1.0
		}
		return
	}

	for _, c := range chunks {
		c.Score = (c.Score - minScore) / diff
	}
}

// linearWeightFusion 线性加权融合
func (h *HybridRetriever) linearWeightFusion(vecChunks, kwChunks []*domain.Chunk, limit int) []*domain.Chunk {
	fusionMap := make(map[string]*domain.Chunk)

	// 处理向量召回结果
	for _, c := range vecChunks {
		chunkCopy := *c // 浅拷贝避免污染原对象
		chunkCopy.Score = chunkCopy.Score * h.alpha
		fusionMap[c.ID] = &chunkCopy
	}

	// 处理关键词召回结果
	for _, c := range kwChunks {
		if existing, ok := fusionMap[c.ID]; ok {
			// 如果该 Chunk 在两路都被召回，分数相加
			existing.Score += c.Score * h.beta
		} else {
			chunkCopy := *c
			chunkCopy.Score = chunkCopy.Score * h.beta
			fusionMap[c.ID] = &chunkCopy
		}
	}

	// 转为切片并按最终融合得分排序
	var result []*domain.Chunk
	for _, c := range fusionMap {
		result = append(result, c)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	if len(result) > limit {
		return result[:limit]
	}
	return result
}
