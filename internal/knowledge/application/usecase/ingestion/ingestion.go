package ingestion

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"
	"llm-agent-platform/internal/knowledge/domain"
	"llm-agent-platform/internal/knowledge/repository"
)

type IngestionUsecase struct {
	embeddingSvc repository.EmbeddingService
	vectorRepo   repository.VectorRepo
	keywordRepo  repository.KeywordRepo
	metaRepo     repository.MetadataRepo
	chunker      Chunker
	logger       *zap.Logger
}

type Chunker interface {
	Chunk(ctx context.Context, doc *domain.Document) ([]*domain.Chunk, error)
}

// SimpleChunker 可替换为 SemanticChunker / RecursiveCharacterTextSplitter
type SimpleChunker struct {
	ChunkSize    int
	ChunkOverlap int
}

func NewSimpleChunker(size, overlap int) *SimpleChunker {
	return &SimpleChunker{ChunkSize: size, ChunkOverlap: overlap}
}

func (s *SimpleChunker) Chunk(ctx context.Context, doc *domain.Document) ([]*domain.Chunk, error) {
	text := doc.Content
	var chunks []*domain.Chunk
	for i := 0; i < len(text); i += s.ChunkSize - s.ChunkOverlap {
		end := i + s.ChunkSize
		if end > len(text) {
			end = len(text)
		}
		chunkID := fmt.Sprintf("%s_chunk_%d", doc.ID, i)
		chunks = append(chunks, &domain.Chunk{
			ID:         chunkID,
			DocumentID: doc.ID,
			Content:    text[i:end],
			Metadata:   doc.Metadata,
		})
	}
	return chunks, nil
}

func NewIngestionUsecase(
	es repository.EmbeddingService,
	vr repository.VectorRepo,
	kr repository.KeywordRepo,
	mr repository.MetadataRepo,
	chunker Chunker,
	logger *zap.Logger,
) *IngestionUsecase {
	return &IngestionUsecase{
		embeddingSvc: es,
		vectorRepo:   vr,
		keywordRepo:  kr,
		metaRepo:     mr,
		chunker:      chunker,
		logger:       logger,
	}
}

// Ingest 完整向量化+多路存储（并发）
func (u *IngestionUsecase) Ingest(ctx context.Context, doc *domain.Document) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context canceled: %w", ctx.Err())
	}

	chunks, err := u.chunker.Chunk(ctx, doc)
	if err != nil || len(chunks) == 0 {
		return err
	}

	// 批量Embedding
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Content
	}
	vectors, err := u.embeddingSvc.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("embedding failed: %w", err)
	}

	// 并发三路存储
	var wg sync.WaitGroup
	var vecErr, kwErr, metaErr error
	wg.Add(3)
	go func() { defer wg.Done(); vecErr = u.vectorRepo.InsertVectors(ctx, chunks, vectors) }()
	go func() { defer wg.Done(); kwErr = u.keywordRepo.IndexKeywords(ctx, chunks) }()
	go func() { defer wg.Done(); metaErr = u.metaRepo.BatchSaveChunks(ctx, chunks) }()
	wg.Wait()

	if vecErr != nil || kwErr != nil || metaErr != nil {
		return fmt.Errorf("storage failed: vec=%v kw=%v meta=%v", vecErr, kwErr, metaErr)
	}

	u.logger.Info("ingestion success", zap.String("doc_id", doc.ID), zap.Int("chunks", len(chunks)))
	return nil
}
