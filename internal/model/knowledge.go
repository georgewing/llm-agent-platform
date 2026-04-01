package model

import (
	"time"

	"gorm.io/datatypes"
)

type Document struct {
	ID        string `gorm:"primaryKey"`
	TenantID  string `gorm:"index"`
	Name      string
	Content   string
	Metadata  string
	Status    string `gorm:"default:'pending'"` // pending, processing, completed, failed
	CreatedAt time.Time
}

type Chunk struct {
	ID         string
	DocumentID string
	TenantID   string
	Content    string
	Embedding  []float32 `gorm:"-"` // 只存Milvus
	Metadata   datatypes.JSON
	CreatedAt  time.Time
}

type RetrievalResult struct {
	ChunkId string
	Score   float64
	Content string
	Source  string
}
