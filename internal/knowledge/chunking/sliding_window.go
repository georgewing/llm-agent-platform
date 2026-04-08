package chunking

import (

	"llm-agent-platform/internal/knowledge/domain"
	"llm-agent-platform/internal/shared/kernel"
)

type SlidingWindowChunker struct {
	config ChunkConfig
}

// ChunkConfig 分块配置
type ChunkConfig struct {
	ChunkSize       int  `json:"chunk_size"`       // 目标块大小
	OverlapSize     int  `json:"overlap_size"`     // 重叠大小
	MinChunkSize    int  `json:"min_chunk_size"`   // 最小块大小
	MaxChunkSize    int  `json:"max_chunk_size"`   // 最大块大小
	RespectBoundary bool `json:"respect_boundary"` // 是否尊重语义边界
}

func NewSlidingWindowChunker(config ChunkConfig) *SlidingWindowChunker {
	return &SlidingWindowChunker{
		config: config,
	}
}

// Chunk 对文档进行分块
func (c *SlidingWindowChunker) Chunk(doc *domain.Document) ([]*domain.Chunk, error) {
	if len(doc.Content) == 0 {
		return nil,
	}
}