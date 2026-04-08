package chunking

import (
	"context"
	"fmt"
	"unicode"

	"llm-agent-platform/internal/knowledge/domain"
)

type SlidingWindowChunker struct {
	config ChunkConfig
}

// ChunkConfig 分块配置
type ChunkConfig struct {
	ChunkSize       int      `json:"chunk_size"`     // 目标块大小
	OverlapSize     int      `json:"overlap_size"`   // 重叠大小
	MinChunkSize    int      `json:"min_chunk_size"` // 最小块大小
	MaxChunkSize    int      `json:"max_chunk_size"` // 最大块大小
	Separators      []string `json:"separators"`     // 分隔符
	RespectBoundary bool     `json:"respect_boundary"`
	Threshold       float64  `json:"threshold"`
}

func NewSlidingWindowChunker(config ChunkConfig) *SlidingWindowChunker {
	// 默认值处理
	if config.ChunkSize <= 0 {
		config.ChunkSize = 512
	}
	if config.MaxChunkSize <= 0 {
		config.MaxChunkSize = config.ChunkSize
	}
	if config.MinChunkSize <= 0 {
		config.MinChunkSize = config.ChunkSize / 4
	}
	if config.OverlapSize < 0 || config.OverlapSize >= config.ChunkSize {
		config.OverlapSize = config.ChunkSize / 10
	}
	return &SlidingWindowChunker{
		config: config,
	}
}

// Chunk 对文档进行分块
func (c *SlidingWindowChunker) Chunk(ctx context.Context, doc *domain.Document) ([]*domain.Chunk, error) {
	if c.config.RespectBoundary {
		// TODO: Implement semantic boundary respect
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if doc == nil || doc.Content == "" {
		return nil, nil
	}

	runes := []rune(doc.Content)
	length := len(runes)

	var chunks []*domain.Chunk

	for start := 0; start < length; {
		// 计算块的理想的结束位置
		idealEnd := start + c.config.ChunkSize
		if idealEnd > length {
			idealEnd = length
		}

		// 语义边界感知：寻找最佳切割点
		end := idealEnd
		if c.config.RespectBoundary && idealEnd < length {
			end = c.findSemanticBoundary(runes, start, idealEnd)
		}
		// 确保块大小在有效范围内
		chunkLen := end - start
		if chunkLen > c.config.MaxChunkSize {
			end = start + c.config.MaxChunkSize
		}
		if end > length {
			end = length
		}

		content := string(runes[start:end])

		// 过滤过短的尾块
		if len([]rune(content)) < c.config.MinChunkSize && len(chunks) > 0 {
			break
		}

		chunkID := fmt.Sprintf("%s_sw_%d", doc.ID, len(chunks))
		chunks = append(chunks, &domain.Chunk{
			ID:         chunkID,
			DocumentID: doc.ID,
			Content:    content,
			Metadata:   doc.Metadata,
		})

		if end == length {
			break
		}

		// 下一步起始位置（考虑重叠）
		start = end - c.config.OverlapSize
		if start >= end {
			start = end
		}
	}

	return chunks, nil
}

// findSemanticBoundary 寻找最佳语义边界
func (c *SlidingWindowChunker) findSemanticBoundary(runes []rune, start, idealEnd int) int {
	// 搜索范围：理想位置前后各 1/4 窗口
	searchRadius := c.config.ChunkSize / 4

	forwardStart := max(start, idealEnd-searchRadius)
	backwardLimit := min(len(runes), idealEnd+c.config.ChunkSize/8)

	// 优先在理想位置之前找边界（保证不超出目标大小）
	bestPos := idealEnd
	bestScore := 0

	// 向前搜索（优先方向）
	for i := idealEnd; i > forwardStart; i-- {
		score := c.scoreBoundary(runes, i)
		dist := idealEnd - i
		if score > bestScore || (score == bestScore && score > 0 && dist < idealEnd-bestPos) {
			bestScore = score
			bestPos = i
		}
	}

	// 向后搜索（需分数优势）
	for i := idealEnd + 1; i < backwardLimit; i++ {
		score := c.scoreBoundary(runes, i)
		if score >= bestScore+10 {
			bestScore = score
			bestPos = i
		}
	}

	// 确保至少达到最小块大小
	if bestPos-start < c.config.MinChunkSize {
		return idealEnd
	}

	return bestPos
}

// scoreBoundary 评估边界质量，返回分数（越高越好）
func (c *SlidingWindowChunker) scoreBoundary(runes []rune, pos int) int {
	if pos <= 0 || pos >= len(runes) {
		return 0
	}

	prev := runes[pos-1]
	next := runes[pos]

	// 段落边界（两个换行）
	if pos >= 2 && runes[pos-2] == '\n' && prev == '\n' {
		return 100
	}

	// 句子边界（中文）
	switch prev {
	case '。', '！', '？':
		if unicode.IsSpace(next) {
			return 95
		}
		return 90
	}
	// 句子边界（英文）
	switch prev {
	case '.', '!', '?':
		if unicode.IsSpace(next) {
			return 85
		}
		return 80
	}

	// 短语边界（中文）
	switch prev {
	case '，', '、', '；', '：':
		return 60
	}

	// 短语边界（英文）
	switch prev {
	case ',', ';', ':':
		return 50
	}

	// 单词边界
	if unicode.IsSpace(prev) && !unicode.IsSpace(next) {
		return 30
	}

	return 0
}
