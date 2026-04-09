package milvus

import (
	"context"
	"fmt"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

type CollectionManager struct {
	client client.Client
}

func NewCollectionManager(c client.Client) *CollectionManager {
	return &CollectionManager{client: c}
}

// CreateCollection 创建 Milvus Collection
func (m *CollectionManager) CreateCollection(ctx context.Context, name string, dim int) error {
	// 定义 Schema
	schema := &entity.Schema{
		CollectionName: name,
		Fields: []*entity.Field{
			{
				Name:       "id",
				DataType:   entity.FieldTypeVarChar,
				PrimaryKey: true,
				AutoID:     false,
				TypeParams: map[string]string{"max_length": "512"},
			},
			{
				Name:     "vector",
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					"dim": fmt.Sprintf("%d", dim),
				},
			},
		},
	}

	// 创建 Collection
	err := m.client.CreateCollection(ctx, schema, 2) // 2 shards
	if err != nil {
		return fmt.Errorf("create collection failed: %w", err)
	}

	// 创建索引
	idx, err := entity.NewIndexHNSW(entity.COSINE, 16, 200)
	if err != nil {
		return fmt.Errorf("create index failed: %w", err)
	}

	err = m.client.CreateIndex(ctx, name, "vector", idx, false)
	if err != nil {
		return fmt.Errorf("create index failed: %w", err)
	}

	// 加载 Collection
	err = m.client.LoadCollection(ctx, name, false)
	if err != nil {
		return fmt.Errorf("load collection failed: %w", err)
	}

	return nil
}

// HasCollection 检查 Collection 是否存在
func (m *CollectionManager) HasCollection(ctx context.Context, name string) (bool, error) {
	return m.client.HasCollection(ctx, name)
}
