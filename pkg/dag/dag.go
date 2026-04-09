package dag

import (
	"context"

	"llm-agent-platform/internal/agent/domain"
)

type Node interface {
	ID() string
	Type() domain.NodeType
	Execute(ctx context.Context, input map[string]any) (map[string]any, error)
	Dependencies() []string
}

type DAG struct {
	Nodes map[string]Node
	Edges map[string][]string // 依赖关系
}

type Executor struct {
	maxWorkers int
	// eventBus *EventBus
}

// 简单实现（后续可替换为真实 worker pool）
type workerPool struct {
	sem chan struct{}
}

func newWorkerPool(maxWorkers int) *workerPool {
	return &workerPool{sem: make(chan struct{}, maxWorkers)}
}

func (p *workerPool) Submit(f func()) {
	p.sem <- struct{}{}
	go func() {
		defer func() { <-p.sem }()
		f()
	}()
}
