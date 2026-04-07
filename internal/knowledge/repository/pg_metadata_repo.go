package repository

import (
	"context"
	"llm-agent-platform/internal/knowledge/domain"
	"llm-agent-platform/internal/model"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type VectorRepo interface {
	SearchVector(ctx context.Context, vector []float32, topK int) ([]*domain.Chunk, error)
}

type KeywordRepo interface {
	SearchKeyword(ctx context.Context, query string, topK int) ([]*domain.Chunk, error)
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
	// TODO: 需要在 model.Document 中添加 Status 字段
	// 或者创建单独的 DocumentStatus 模型
	return r.db.WithContext(ctx).Model(&model.Document{}).
		Where("id = ?", docID).
		Update("status", status).Error
}

// BatchSaveChunks 批量落库 Chunk 的纯文本和元数据 (不存向量)
func (r *PGMetadataRepo) BatchSaveChunks(ctx context.Context, chunks []*domain.Chunk) error {
	return r.db.WithContext(ctx).Create(&chunks).Error
}

// GetChunksByIDs 批量回表查询
func (r *PGMetadataRepo) GetChunksByIDs(ctx context.Context, ids []string) (map[string]*domain.Chunk, error) {
	var chunks []*domain.Chunk

	// GORM 批量查询： SELECT * FROM chunks WHERE id IN (?)
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&chunks).Error
	if err != nil {
		return nil, err
	}

	// 转为 Map 以便 O(1) 复杂度回填
	chunkMap := make(map[string]*domain.Chunk)
	for _, c := range chunks {
		domainChunk := r.modelToDomainChunk(&c)
		chunkMap[c.ID] = domainChunk
	}
	return chunkMap, nil
}

func (r *PGMetadataRepo) domainToModelDocument(doc *domain.Document) *model.Document {
	return &model.Document{
		ID:        doc.ID,
		TenantID:  "default", // TODO: 从上下文获取租户 ID
		Name:      doc.Title,
		Content:   doc.Content,
		Metadata:  doc.Metadata,
		CreatedAt: doc.CreatedAt,
	}
}

func (r *PGMetadataRepo) modelToDomainDocument(doc *model.Document) *domain.Document {
	return &domain.Document{
		ID:        doc.ID,
		Title:     doc.Name,
		Content:   doc.Content,
		Metadata:  doc.Metadata,
		CreatedAt: doc.CreatedAt,
		UpdatedAt: doc.CreatedAt, // TODO: model 中需要添加 UpdatedAt 字段
	}
}

func (r *PGMetadataRepo) domainToModelChunk(chunk *domain.Chunk) *model.Chunk {
	return &model.Chunk{
		ID:         chunk.ID,
		DocumentID: chunk.DocumentID,
		TenantID:   "default", // TODO: 从上下文获取租户 ID
		Embedding:  nil,       // 不存 PG，只存 Milvus
		Metadata:   datatypes.JSON(chunk.Metadata),
	}
}

func (r *PGMetadataRepo) modelToDomainChunk(chunk *model.Chunk) *domain.Chunk {
	return &domain.Chunk{
		ID:         chunk.ID,
		DocumentID: chunk.DocumentID,
		Content:    chunk.Content,
		Metadata:   chunk.Metadata,
		Score:      0, // 数据库中没有分数，分数是检索时计算的
	}
}
