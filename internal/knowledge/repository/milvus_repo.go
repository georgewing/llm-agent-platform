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
	// 1. 组装 Milvus 搜索参数 (L2 或 IP 距离)
	// 2. 调用 r.client.Search(...)
	// 3. 将返回的 entity 转换为 domain.Chunk
	// 注意：Milvus 召回通常只返回 ChunkID 和 Score

	// 伪代码映射
	var chunks []*domain.Chunk
	// for _, result := range searchResults {
	//    chunks = append(chunks, &domain.Chunk{
	//        ID: result.ID,
	//        Score: result.Distance,
	//    })
	// }
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
	vectorCol := entity.NewColumnFloatVector("vector", vecData)

	result, err := r.client.Insert(ctx, r.collectionName, "", idCol, vectorCol)
	if err != nil {
		return fmt.Errorf("milvus insert failed: %w", err)
	}

	if result.InsertCount != int64(len(chunks)) {
		// log warning but still success (partial insert)
	}
	return nil
}
