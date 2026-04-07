package domain

type Embeddings struct {
	Model     string    `json:"model"`
	Dimension int       `json:"dimension"`
	Vector    []float32 `json:"vector"`
	Tokens    int       `json:"tokens"`
}

// 工厂方法
func NewEmbeddings(model string, vector []float32, tokens int) *Embeddings {
	return &Embeddings{
		Model:     model,
		Dimension: len(vector),
		Vector:    vector,
		Tokens:    tokens,
	}
}
