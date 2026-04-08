package chunking

import (
	"context"
	"fmt"
	"strings"

	"llm-agent-platform/internal/knowledge/domain"
)

type RecursiveCharacterChunker struct {
	ChunkSize    int
	ChunkOverlap int
	Separators   []string // 支持多层分隔符，优先级从高到低
}

func NewRecursiveCharacterChunker(config ChunkConfig) *RecursiveCharacterChunker {
	if config.ChunkSize <= 0 {
		config.ChunkSize = 512
	}
	if config.OverlapSize < 0 || config.OverlapSize >= config.ChunkSize {
		config.OverlapSize = config.ChunkSize / 10
	}
	separators := config.Separators
	if len(separators) == 0 {
		separators = []string{"\n\n", "\n", "。", "！", "？", ".", " ", ""}
	}

	return &RecursiveCharacterChunker{
		ChunkSize:    config.ChunkSize,
		ChunkOverlap: config.OverlapSize,
		Separators:   separators,
	}
}

func (c *RecursiveCharacterChunker) Chunk(ctx context.Context, doc *domain.Document) ([]*domain.Chunk, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if doc == nil || doc.Content == "" {
		return nil, nil
	}

	text := strings.TrimSpace(doc.Content)
	chunksText := c.recursiveSplit(text, 0)

	var chunks []*domain.Chunk
	for i, content := range chunksText {
		chunkID := fmt.Sprintf("%s_chunk_%d", doc.ID, i)
		chunks = append(chunks, &domain.Chunk{
			ID:         chunkID,
			DocumentID: doc.ID,
			Content:    strings.TrimSpace(content),
			Metadata:   doc.Metadata, // 继承文档元数据
		})
	}
	return chunks, nil
}

// 递归拆分核心逻辑（LangChain 经典实现）
func (c *RecursiveCharacterChunker) recursiveSplit(text string, sepIdx int) []string {
	if len([]rune(text)) <= c.ChunkSize {
		return []string{text}
	}
	if sepIdx >= len(c.Separators) {
		// 最后一层按字符切
		return c.splitFixed(text)
	}

	sep := c.Separators[sepIdx]
	parts := strings.Split(text, sep)
	if len(parts) == 1 {
		// 当前分隔符无效 → 进入下一层
		return c.recursiveSplit(text, sepIdx+1)
	}

	var chunks []string
	var current strings.Builder

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 当前 chunk 已满，先保存
		if current.Len()+len(part)+len(sep) > c.ChunkSize {
			if current.Len() > 0 {
				chunks = append(chunks, current.String())
			}
			current.Reset()
		}

		// 加入 overlap（前一个 chunk 尾部内容）
		if len(chunks) > 0 && c.ChunkOverlap > 0 {
			last := chunks[len(chunks)-1]
			if len(last) > c.ChunkOverlap {
				current.WriteString(last[len(last)-c.ChunkOverlap:])
				current.WriteString(sep)
			}
		}

		if current.Len() > 0 {
			current.WriteString(sep)
		}
		current.WriteString(part)
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

func (c *RecursiveCharacterChunker) splitFixed(text string) []string {
	var chunks []string
	for i := 0; i < len([]rune(text)); i += c.ChunkSize - c.ChunkOverlap {
		end := i + c.ChunkSize
		if end > len([]rune(text)) {
			end = len([]rune(text))
		}
		chunks = append(chunks, text[i:end])
	}
	return chunks
}
