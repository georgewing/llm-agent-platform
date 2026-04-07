package retrieval

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"go.uber.org/zap"
	"llm-agent-platform/internal/knowledge/domain"
	"llm-agent-platform/internal/knowledge/repository"
)

// HybridRetriever 混合检索器
type HybridRetriever struct {
	vectorRepo  repository.VectorRepo   // Milvus: 返回 ID + 距离
	keywordRepo repository.KeywordRepo  // ES: 返回 ID + BM25分数 (+可能包含Content)
	metaRepo    repository.MetadataRepo // PG: 根据 ID 补全 Content 和 Metadata
	reranker    Reranker                // 重排器
	alpha       float64
	beta        float64
	logger      *zap.Logger
}

// NewHybridRetriever 初始化混合检索器 (新增了 MetadataRepo 依赖)
func NewHybridRetriever(
	vr repository.VectorRepo,
	kr repository.KeywordRepo,
	mr repository.MetadataRepo, // 注入 PG Repo
	reranker Reranker,
	alpha, beta float64,
	logger *zap.Logger,
) *HybridRetriever {
	return &HybridRetriever{
		vectorRepo:  vr,
		keywordRepo: kr,
		metaRepo:    mr,
		reranker:    reranker,
		alpha:       alpha,
		beta:        beta,
		logger:      logger,
	}
}

// Retrieve 完整的工业级 RAG 检索 Pipeline
func (h *HybridRetriever) Retrieve(ctx context.Context, query string, queryVector []float32, topK int) ([]*domain.Chunk, error) {
	// 上下文取消检查
	if ctx.Err() != nil {
		return nil, fmt.Errorf("上下文取消: %w", ctx.Err())
	}

	var (
		wg        sync.WaitGroup
		vecChunks []*domain.Chunk
		kwChunks  []*domain.Chunk
		vecErr    error
		kwErr     error
	)

	// 阶段 1：并发双路召回 (获取 Chunk ID 和 原始 Score)
	wg.Add(2)
	go func() {
		defer wg.Done()
		// Milvus 召回：此时 Chunk 里面只有 ID 和 Score，没有 Content
		vecChunks, vecErr = h.vectorRepo.SearchVector(ctx, queryVector, topK)
	}()
	go func() {
		defer wg.Done()
		// ES 召回：包含 ID 和 Score
		kwChunks, kwErr = h.keywordRepo.SearchKeyword(ctx, query, topK)
	}()
	wg.Wait()

	if vecErr != nil && kwErr != nil {
		return nil, fmt.Errorf("both vector and keyword search failed: %w", vecErr) // 两路全挂才报错
	}

	// 阶段 2：分数归一化与线性加权融合 (只计算分数)
	h.normalizeScores(vecChunks)
	h.normalizeScores(kwChunks)

	// 融合后的结果，按照综合得分降序，保留 Top N (通常 N = topK * 2 给重排留出空间)
	fusedChunks := h.linearWeightFusion(vecChunks, kwChunks, topK*2)

	if len(fusedChunks) == 0 {
		return []*domain.Chunk{}, nil
	}

	// 阶段 3：回表 (Hydration) - 从 PG 查询完整文本
	// 提取出所有融合后的 Chunk ID
	var chunkIDs []string
	for _, c := range fusedChunks {
		chunkIDs = append(chunkIDs, c.ID)
	}

	// 批量查询 PG 获取真实的 Content 和 Metadata
	// 注意：在 MetadataRepo 中需要实现 GetChunksByIDs(ctx,[]string) 方法
	pgChunksMap, err := h.metaRepo.GetChunksByIDs(ctx, chunkIDs)
	if err != nil {
		return nil, fmt.Errorf("metadata hydration failed: %w", err) // 如果 PG 挂了，无法获取文本，只能报错
	}

	// 将 PG 的完整数据组装（回填）到 FusedChunks 中
	var hydratedChunks []*domain.Chunk
	for _, c := range fusedChunks {
		if pgData, exists := pgChunksMap[c.ID]; exists {
			c.Content = pgData.Content
			c.Metadata = pgData.Metadata
			// 保留 c.Score (这个是融合后的检索分数)
			hydratedChunks = append(hydratedChunks, c)
		}
	}

	// 阶段 4：重排 (降级友好)
	// Reranker 必须拿到 hydratedChunks，因为它需要读取 c.Content
	if h.reranker != nil && len(hydratedChunks) > 0 {
		reranked, err := h.reranker.Rerank(ctx, query, hydratedChunks, topK)
		if err == nil {
			h.logger.Info("hybrid retrieve reranked", zap.Int("original", len(hydratedChunks)), zap.Int("final", len(reranked)))
			return reranked, nil
		}
		// 如果大模型重排超时或失败，降级，继续往下走直接返回融合结果
		h.logger.Warn("reranking failed", zap.Error(err))
	}

	// 阶段 5：截断返回最终结果
	if len(hydratedChunks) > topK {
		hydratedChunks = hydratedChunks[:topK]
	}
	h.logger.Info("hybrid retrieve completed", zap.String("query", query), zap.Int("final", len(hydratedChunks)))
	return hydratedChunks, nil
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
