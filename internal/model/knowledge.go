package model

import "time"

type Document struct {
	ID        string `gorm:"primaryKey"`
	TenantID  string `gorm:"index"`
	Name      string
	Content   string
	Metadata  string
	CreatedAt time.Time
}

type Chunk struct {
	ID         string
	DocumentID string
	TenantID   string
	Embedding  []float32 `gorm:"-"` // 只存Milvus
	Metadata   datatypes.JSON
}

type RetrievalResult struct {
	ChunkId string
	Score   float64
	Content string
	Source  string
}
