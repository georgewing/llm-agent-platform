package domain

import (
	"time"
)

type Chunk struct {
	ID         string                 `json:"id"`
	Content    string                 `json:"content"`
	Metadata   map[string]interface{} `json:"metadata"`
	Score      float64                `json:"score"`
	Embedding  *Embedding             `json:"embedding"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
	DocumentID string                 `json:"document_id"`
	TenantID   string                 `json:"tenant_id"`
}

// NewChunk 工厂方法
func NewChunk(id string, content string, metadata map[string]interface{}) *Chunk {
	now := time.Now()
	return &Chunk{
		ID:        id,
		Content:   content,
		Metadata:  metadata,
		Score:     0.0,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
