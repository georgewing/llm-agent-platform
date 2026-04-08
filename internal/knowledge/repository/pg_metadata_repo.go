package repository

import (
	"context"
	"llm-agent-platform/internal/knowledge/domain"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type EmbeddingService interface {
	EmbedBatch(ctx context.Context, text []string) ([][]float32, error)
}

type VectorRepo interface {
	SearchVector(ctx context.Context, vector []float32, topK int) ([]*domain.Chunk, error)
	InsertVectors(ctx context.Context, chunks []*domain.Chunk, vector [][]float32) error
}

type KeywordRepo interface {
	SearchKeyword(ctx context.Context, query string, topK int) ([]*domain.Chunk, error)
	IndexKeywords(ctx context.Context, chunks []*domain.Chunk) error
}

type MetadataRepo interface {
	CreateDocument(ctx context.Context, doc *domain.Document) error
	UpdateDocumentStatus(ctx context.Context, docID string, status string) error
	BatchSaveChunks(ctx context.Context, chunks []*domain.Chunk) error
	GetChunksByIDs(ctx context.Context, ids []string) (map[string]*domain.Chunk, error)
}

type PGMetadataRepo struct {
	db *gorm.DB
}

func NewPGMetadataRepo(db *gorm.DB) *PGMetadataRepo {
	return &PGMetadataRepo{db: db}
}

func (r *PGMetadataRepo) CreateDocument(ctx context.Context, doc *domain.Document) error {
	// 领域模型转换为 GORM 模型
	modelDoc := r.domainToModelDocument(doc)

	return r.db.WithContext(ctx).Create(modelDoc).Error
}

func (r *PGMetadataRepo) UpdateDocumentStatus(ctx context.Context, docID string, status string) error {
	// 或者创建单独的 DocumentStatus 模型
	return r.db.WithContext(ctx).Model(&domain.Document{}).
		Where("id = ?", docID).
		Update("status", status).Error
}

// BatchSaveChunks 批量落库 Chunk 的纯文本和元数据 (不存向量)
func (r *PGMetadataRepo) BatchSaveChunks(ctx context.Context, chunks []*domain.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	modelChunks := make([]*domain.Chunk, len(chunks))
	for i, chunk := range chunks {
		modelChunks[i] = r.domainToModelChunk(chunk)
	}
	return r.db.WithContext(ctx).Create(&modelChunks).Error
}

// GetChunksByIDs 批量回表查询
func (r *PGMetadataRepo) GetChunksByIDs(ctx context.Context, ids []string) (map[string]*domain.Chunk, error) {
	if len(ids) == 0 {
		return make(map[string]*domain.Chunk), nil
	}
	var modelChunks []*domain.Chunk
	err := r.db.WithContext(ctx).
		Where("id IN ?", ids).
		Find(&modelChunks).Error
	if err != nil {
		return nil, err
	}

	// 转为 Map 以便 O(1) 复杂度回填
	chunkMap := make(map[string]*domain.Chunk)
	for _, mc := range modelChunks {
		domainChunk := r.modelToDomainChunk(mc)
		chunkMap[mc.ID] = domainChunk
	}
	return chunkMap, nil
}

func (r *PGMetadataRepo) domainToModelDocument(doc *domain.Document) *domain.Document {
	return &domain.Document{
		ID:        doc.ID,
		TenantID:  "default", // TODO: 从上下文获取租户 ID
		Name:      doc.Title,
		Content:   doc.Content,
		Metadata:  doc.Metadata,
		CreatedAt: doc.CreatedAt,
	}
}

func (r *PGMetadataRepo) modelToDomainDocument(doc *domain.Document) *domain.Document {
	return &domain.Document{
		ID:        doc.ID,
		Title:     doc.Name,
		Content:   doc.Content,
		Metadata:  doc.Metadata,
		CreatedAt: doc.CreatedAt,
		UpdatedAt: doc.CreatedAt,
	}
}

func (r *PGMetadataRepo) domainToModelChunk(chunk *domain.Chunk) *domain.Chunk {
	return &domain.Chunk{
		ID:         chunk.ID,
		DocumentID: chunk.DocumentID,
		TenantID:   "default", // TODO: 从上下文获取租户 ID
		Content:    chunk.Content,
		Embedding:  nil, // 不存 PG，只存 Milvus
		Metadata:   datatypes.JSON(chunk.Metadata),
	}
}

func (r *PGMetadataRepo) modelToDomainChunk(chunk *domain.Chunk) *domain.Chunk {
	return &domain.Chunk{
		ID:         chunk.ID,
		DocumentID: chunk.DocumentID,
		Content:    chunk.Content,
		Metadata:   chunk.Metadata,
		Score:      0, // 数据库中没有分数，分数是检索时计算的
	}
}
