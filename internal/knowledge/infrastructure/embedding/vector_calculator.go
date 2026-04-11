package embedding

import (
	"errors"
	"math"

	"llm-agent-platform/internal/knowledge/domain"
)

type CosineVectorCalculator struct{}

func NewCosineVectorCalculator() *CosineVectorCalculator {
	return &CosineVectorCalculator{}
}

var _ domain.VectorCalculator = (*CosineVectorCalculator)(nil)

func (c *CosineVectorCalculator) CosineSimilarity(v1, v2 []float32) (float32, error) {

	var dot, norm1, norm2 float64
	for i := range v1 {
		a := float64(v1[i])
		b := float64(v2[i])
		dot += a * b
		norm1 += a * a
		norm2 += b * b
	}

	if norm1 == 0 || norm2 == 0 {
		return 0, errors.New("零向量无法计算余弦相似度")
	}

	sim := dot / (math.Sqrt(norm1) * math.Sqrt(norm2))

	// 防止浮点误差导致溢出 [-1, 1] 界限
	if sim > 1 {
		sim = 1
	} else if sim < -1 {
		sim = -1
	}

	return float32(sim), nil
}
