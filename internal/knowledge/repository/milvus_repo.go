package repository

import (
	"context"
	"fmt"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"llm-agent-platform/internal/knowledge/domain"
)

type MilvusRepo struct {
	client         client.Client
	collectionName string
}

func NewMilvusRepo(c client.Client, collName string) *MilvusRepo {
	return &MilvusRepo{client: c, collectionName: collName}
}

// SearchVector 实现基于 IVF_FLAT/HNSW 的向量召回
func (r *MilvusRepo) SearchVector(ctx context.Context, vector []float32, topK int) ([]*domain.Chunk, error) {
	if len(vector) == 0 {
		return nil, fmt.Errorf("vector is empty")
	}

	// 1. 组装 Milvus 搜索参数 (L2 或 IP 距离)
	searchParam, err := entity.NewIndexHNSWSearchParam(64)
	if err != nil {
		return nil, fmt.Errorf("new search param failed: %w", err)
	}
	searchParam.WithMetricType(entity.COSINE) // 推荐：余弦相似度（Embedding 通常已归一化）
	// 2. 执行向量搜索（单向量查询）
	searchResults, err := r.client.Search(
		ctx,
		r.collectionName,
		[]string{}, // partitions（空 = 全 collection）
		"",         // filter expression（可后续扩展）
		[]string{}, // outputFields 只返回 PK + score（Content 不存 Milvus）
		[]entity.Vector{entity.FloatVector(vector)},
		"vector",      // 向量字段名（与 InsertVectors 一致）
		entity.COSINE, // metricType（必须与 collection schema 匹配）
		topK,
		searchParam,
	)
	if err != nil {
		return nil, fmt.Errorf("milvus search failed: %w", err)
	}

	// 3. 将返回的 entity 转换为 domain.Chunk
	// 注意：Milvus 召回通常只返回 ChunkID 和 Score
	var chunks []*domain.Chunk
	for _, result := range searchResults {
		// result.IDs 和 result.Scores 长度一致
		for i := range result.IDs {
			chunkID, ok := result.IDs[i].(string)
			if !ok {
				continue // 类型不匹配，跳过
			}
			score := result.Scores[i]

			chunks = append(chunks, &domain.Chunk{
				ID:    chunkID,
				Score: float64(score), // COSINE 时为相似度（越高越好）；L2 时为距离（越低越好）
				// Content 留空，后续通过 GetChunksByIDs 从 PG 回填
			})
		}
	}

	return chunks, nil
}

func (r *MilvusRepo) InsertVectors(ctx context.Context, chunks []*domain.Chunk, vectors [][]float32) error {
	if len(chunks) == 0 || len(chunks) != len(vectors) {
		return fmt.Errorf("chunks and vectors length mismatch")
	}

	ids := make([]string, len(chunks))
	vecData := make([][]float32, len(chunks))
	for i, chunk := range chunks {
		ids[i] = chunk.ID
		vecData[i] = vectors[i]
	}

	idCol := entity.NewColumnVarChar("id", ids)
	dim := len(vecData[0])
	vectorCol := entity.NewColumnFloatVector("vector", dim, vecData)

	result, err := r.client.Insert(ctx, r.collectionName, "", idCol, vectorCol)
	if err != nil {
		return fmt.Errorf("milvus insert failed: %w", err)
	}

	if result.InsertCount != int64(len(chunks)) {
		// log warning but still success (partial insert)
	}
	return nil
}
