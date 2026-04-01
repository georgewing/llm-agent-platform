package repository

import (
	"context"
	"gorm.io/gorm"
	"llm-agent-platform/internal/knowledge/domain"
)

// MetadataRepo 定义关系型数据接口
type MetadataRepo interface {
	CreateDocument(ctx context.Context, doc *domain.Document) error
	UpdateDocumentStatus(ctx context.Context, docID string, status string) error
	BatchSaveChunks(ctx context.Context, chunks []*domain.Chunk) error
	GetChunkByID(ctx context.Context, chunkID string) (*domain.Chunk, error)
}

type PGMetadataRepo struct {
	db *gorm.DB
}

func NewPGMetadataRepo(db *gorm.DB) *PGMetadataRepo {
	return &PGMetadataRepo{db: db}
}

// BatchSaveChunks 批量落库 Chunk 的纯文本和元数据 (不存向量)
func (r *PGMetadataRepo) BatchSaveChunks(ctx context.Context, chunks []*domain.Chunk) error {
	return r.db.WithContext(ctx).Create(&chunks).Error
}
