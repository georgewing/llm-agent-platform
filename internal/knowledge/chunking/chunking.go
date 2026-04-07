package chunking

import (
	"context"

	"llm-agent-platform/internal/knowledge/domain"
)

// Chunker 是统一的文档分块接口（所有 Chunk 策略必须实现）
// 放在 chunking 包统一管理，便于后续扩展多种分块策略
type Chunker interface {
	Chunk(ctx context.Context, doc *domain.Document) ([]*domain.Chunk, error)
}
