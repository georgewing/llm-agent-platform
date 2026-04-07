package chunking

import (
	"context"
	"fmt"
	"math"
	"strings"

	"llm-agent-platform/internal/knowledge/domain"
	"llm-agent-platform/internal/knowledge/repository"
)

type SemanticChunker struct {
	embeddingSvc repository.EmbeddingService
	ChunkSize    int
	Threshold    float64 // 余弦相似度阈值，低于此值视为语义断崖（推荐 0.2~0.35）
}

func NewSemanticChunker(emb repository.EmbeddingService, chunkSize int, threshold float64) *SemanticChunker {
	if chunkSize <= 0 {
		chunkSize = 512
	}
	if threshold <= 0 || threshold >= 1 {
		threshold = 0.25 // 经验值，平衡 chunk 数量与语义完整性
	}
	return &SemanticChunker{
		embeddingSvc: emb,
		ChunkSize:    chunkSize,
		Threshold:    threshold,
	}
}

func (c *SemanticChunker) Chunk(ctx context.Context, doc *domain.Document) ([]*domain.Chunk, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if doc == nil || doc.Content == "" || c.embeddingSvc == nil {
		return nil, nil
	}

	// 1. 句子级拆分（中英文兼容）
	sentences := c.splitSentences(doc.Content)
	if len(sentences) == 0 {
		return nil, nil
	}

	// 2. 批量 Embedding
	vectors, err := c.embeddingSvc.EmbedBatch(ctx, sentences)
	if err != nil {
		return nil, fmt.Errorf("semantic chunking embed failed: %w", err)
	}
	if len(vectors) != len(sentences) {
		return nil, fmt.Errorf("embedding length mismatch")
	}

	// 3. 寻找语义断崖
	breakPoints := c.findBreakPoints(vectors)

	// 4. 按断点聚合 chunk
	chunksText := c.groupIntoChunks(sentences, breakPoints)

	var chunks []*domain.Chunk
	for i, content := range chunksText {
		chunkID := fmt.Sprintf("%s_semantic_chunk_%d", doc.ID, i)
		chunks = append(chunks, &domain.Chunk{
			ID:         chunkID,
			DocumentID: doc.ID,
			Content:    strings.TrimSpace(content),
			Metadata:   doc.Metadata,
		})
	}
	return chunks, nil
}

// 句子拆分（支持中英文句号、感叹号、问号）
func (c *SemanticChunker) splitSentences(text string) []string {
	text = strings.TrimSpace(text)
	// 统一标点
	text = strings.ReplaceAll(text, "！", "。")
	text = strings.ReplaceAll(text, "？", "。")
	text = strings.ReplaceAll(text, "!", "。")
	text = strings.ReplaceAll(text, "?", "。")
	sents := strings.Split(text, "。")
	var result []string
	for _, s := range sents {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s+"。") // 保留句号
		}
	}
	return result
}

// 计算余弦相似度并找出断点
func (c *SemanticChunker) findBreakPoints(vectors [][]float32) []int {
	breakPoints := []int{0}
	for i := 1; i < len(vectors); i++ {
		sim := cosineSimilarity(vectors[i-1], vectors[i])
		if sim < c.Threshold {
			breakPoints = append(breakPoints, i)
		}
	}
	return breakPoints
}

func cosineSimilarity(a, b []float32) float64 {
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float64(dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB)))))
}

func (c *SemanticChunker) groupIntoChunks(sentences []string, breakPoints []int) []string {
	var chunks []string
	for i := 0; i < len(breakPoints); i++ {
		start := breakPoints[i]
		end := len(sentences)
		if i+1 < len(breakPoints) {
			end = breakPoints[i+1]
		}

		// 聚合句子直到达到 ChunkSize
		var group strings.Builder
		for j := start; j < end; j++ {
			group.WriteString(sentences[j])
			if group.Len() >= c.ChunkSize {
				break
			}
		}
		if group.Len() > 0 {
			chunks = append(chunks, group.String())
		}
	}
	return chunks
}
