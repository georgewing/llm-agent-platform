package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Document struct {
	ID        string    `json:"id"`
	DatasetID string    `json:"dataset_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Status    string    `json:"status"` // PENDING, PROCESSING, COMPLETED, FAILED
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Metadata  datatypes.JSON
	TenantID  string `json:"tenant_id"`
}

// SplitIntoChunks：执行文档分块逻辑
func (d *Document) SplitIntoChunks(chunkSize, overlap int) ([]*Chunk, error) {
	if len(d.Content) == 0 {
		return nil, errors.New("document content is empty")
	}

	var chunks []*Chunk
	runes := []rune(d.Content)
	length := len(runes)

	// 滑动窗口分块算法
	for i := 0; i < length; i += chunkSize - overlap {
		end := i + chunkSize
		if end > length {
			end = length
		}

		chunkContent := string(runes[i:end])
		chunks = append(chunks, &Chunk{
			ID:         uuid.New().String(),
			Content:    chunkContent,
			DocumentID: d.ID,
			CreatedAt:  time.Now(),
		})

		if end == length {
			break
		}
	}

	return chunks, nil
}
