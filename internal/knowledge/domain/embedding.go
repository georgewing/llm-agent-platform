package domain

import (
	"errors"
	"fmt"
)

type Embedding struct {
	Model     string    `json:"model"`
	Dimension int       `json:"dimension"`
	Vector    []float32 `json:"vector"`
	Tokens    int       `json:"tokens"`
}

// 工厂方法
func NewEmbedding(model string, vector []float32, tokens int) (*Embedding, error) {
	if model == "" || len(vector) == 0 {
		return nil, errors.New("model and vector cannot be empty")
	}
	if len(vector) == 0 {
		return nil, errors.New("embedding vector 不能为空")
	}
	if tokens < 0 {
		return nil, errors.New("tokens 不能为负数")
	}

	return &Embedding{
		Model:     model,
		Dimension: len(vector),
		Vector:    vector,
		Tokens:    tokens,
	}, nil
}

// MustNewEmbedding 提供 panic 版本，适合测试或确定不会出错的场景
func MustNewEmbedding(model string, vector []float32, tokens int) *Embedding {
	e, err := NewEmbedding(model, vector, tokens)
	if err != nil {
		panic(fmt.Sprintf("创建 Embedding 失败: %v", err))
	}
	return e
}

func (e *Embedding) GetModel() string     { return e.Model }
func (e *Embedding) GetDimension() int    { return e.Dimension }
func (e *Embedding) GetVector() []float32 { return e.Vector } // 返回副本以防外部修改
func (e *Embedding) GetTokens() int       { return e.Tokens }

// Equals 值对象相等性比较
func (e *Embedding) Equals(other *Embedding) bool {
	if e == nil || other == nil {
		return e == other
	}
	if e.Model != other.Model || e.Dimension != other.Dimension || e.Tokens != other.Tokens {
		return false
	}
	if len(e.Vector) != len(other.Vector) {
		return false
	}
	for i := range e.Vector {
		if e.Vector[i] != other.Vector[i] {
			return false
		}
	}
	return true
}

// Similarity：计算与另一个 Embedding 的余弦相似度
func (e *Embedding) Similarity(other *Embedding, calc VectorCalculator) (float32, error) {
	if e.Dimension != other.Dimension {
		return 0, errors.New("维度不一致，无法计算相似度")
	}

	if calc == nil {
		return 0, errors.New("缺少向量计算器实现")
	}

	// 注入向量计算器
	return calc.CosineSimilarity(e.Vector, other.Vector)
}

// String 便于日志打印
func (e *Embedding) String() string {
	return fmt.Sprintf("Embedding[model=%s, dim=%d, tokens=%d]", e.Model, e.Dimension, e.Tokens)
}
