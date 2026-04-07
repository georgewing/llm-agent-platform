package domain

import "time"

type Chunk struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata"`
	Score     float64                `json:"score"`
	Embedding []float32              `json:"-"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
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
