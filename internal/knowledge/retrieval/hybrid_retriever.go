package retrieval

import (
	"context"
	"sort"
)

type Document struct {
	ID       string
	Content  string
	Score    float64        // RRF融合后的最终得分
	Metadata map[string]any // 包含文档来源、页码等附加信息
}

type RetrievalRequest struct {
	Query string
	TopK  int
}

// RetrievalResult 检索结果
type RetrievalResult struct {
	Documents []*Document
}

type Retriever interface {
	Retrieve(ctx context.Context, req *RetrievalRequest) (*RetrievalResult, error)
}

type HybridRetriever struct {
	vectorRetriever  Retriever
	keywordRetriever Retriever
	rrfK             int // RRF平滑常数，默认通常设为60
}

func (r *HybridRetriever) Retrieve(ctx context.Context, req *RetrievalRequest) (*RetrievalResult, error) {
	var (
		vectorResult  *RetrievalResult
		keywordResult *RetrievalResult
	)
	errChan := make(chan error, 2)

	// 发起并发请求: 向量检索 (Vector Retrieval)
	go func() {
		result, err := r.vectorRetriever.Retrieve(ctx, req)
		if err != nil {
			errChan <- err
		} else {
			vectorResult = result
			errChan <- nil
		}
	}()

	// 发起并发请求: 关键词/全文检索
	go func() {
		result, err := r.keywordRetriever.Retrieve(ctx, req)
		if err != nil {
			errChan <- err
		} else {
			keywordResult = result
			errChan <- nil
		}
	}()

	// 错误处理: 遍历 errChan
	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	// 重排融合: RRF (倒数秩融合算法)
	return r.rrfRank(vectorResult, keywordResult, req.TopK), nil
}

// rrfRank 倒数秩融合算法实现 (Reciprocal Rank Fusion)
func (r *HybridRetriever) rrfRank(vectorResult, keywordResult *RetrievalResult, topK int) *RetrievalResult {
	scores := make(map[string]float64)
	docsMap := make(map[string]*Document)

	// 处理向量检索的排名
	if vectorResult != nil {
		for rank, doc := range vectorResult.Documents {
			scores[doc.ID] += 1.0 / float64(r.rrfK+rank+1)
			docsMap[doc.ID] = doc
		}
	}

	// 处理关键词检索的排名
	if keywordResult != nil {
		for rank, doc := range keywordResult.Documents {
			scores[doc.ID] += 1.0 / float64(r.rrfK+rank+1)
			docsMap[doc.ID] = doc
		}
	}

	// 组装融合后的结果集
	var fusedDocs []*Document
	for id, score := range scores {
		doc := docsMap[id]

		fusedDoc := &Document{
			ID:       doc.ID,
			Content:  doc.Content,
			Metadata: doc.Metadata,
			Score:    score, // 赋予 RRF 融合分数
		}
		fusedDocs = append(fusedDocs, fusedDoc)
	}

	// 按照 Score 降序排序
	sort.Slice(fusedDocs, func(i, j int) bool {
		return fusedDocs[i].Score > fusedDocs[j].Score
	})

	// 截取 Top K 结果并返回
	if len(fusedDocs) > topK {
		fusedDocs = fusedDocs[:topK]
	}
	return &RetrievalResult{Documents: fusedDocs}
}
