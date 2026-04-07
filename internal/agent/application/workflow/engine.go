package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"llm-agent-platform/internal/agent/domain"
)

type DAGEngine struct {
	logger *zap.Logger
}

func NewDAGEngine(logger *zap.Logger) *DAGEngine {
	return &DAGEngine{logger: logger}
}

func (e *DAGEngine) Execute(ctx context.Context, wf *domain.Workflow, inputs map[string]any) (map[string]any, error) {
	start := time.Now()
	e.logger.Info("start execute workflow", zap.String("workflow_id", wf.ID), zap.String("name", wf.Name))

	// 验证 DAG
	if err := e.validateDAG(wf); err != nil {
		return nil, fmt.Errorf("validate dag: %w", err)
	}

	// 拓扑排序 + 并发执行
	g, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	results := make(map[string]domain.NodeOutput)
	results[domain.NodeTypeStart.String()] = inputs

	// 入度表（用于Kahn算法）
	inDegree := make(map[string]int)
	for id := range wf.Nodes {
		inDegree[id] += len(wf.Nodes[id].Dependencies)
	}

	// 就绪队列（buffered channel，防止高并发下阻塞）
	ready := make(chan string, len(wf.Nodes))
	for id, deg := range inDegree {
		if deg == 0 {
			ready <- id
		}
	}

	// 并发执行所有就绪Node
	executed := make(map[string]struct{})
	executedCount := 0

	for executedCount < len(wf.Nodes) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case nodeID := <-ready:
			if _, done := executed[nodeID]; done {
				continue
			}

			node := wf.Nodes[nodeID]
			executed[nodeID] = struct{}{}

			g.Go(func() error {
				defer func() {
					// 触发后续Node
					mu.Lock()
					for id, n := range wf.Nodes {
						for _, def := range n.Dependencies {
							if def == nodeID {
								inDegree[id]--
								if inDegree[id] == 0 {
									ready <- id
								}
							}
						}
					}
					mu.Unlock()
				}()

				// 收集前置依赖输入
				input := e.mergeInputs(node.Dependencies, results, &mu)

				output, err := e.executeNodeWithRetry(ctx, node, input)
				if err != nil {
					e.logger.Error("execute node failed", zap.String("node_id", nodeID), zap.Error(err))
					return fmt.Errorf("execute node %s(%s): %w", nodeID, node.Name, err)
				}

				mu.Lock()
				results[nodeID] = output
				mu.Unlock()

				e.logger.Debug("node completed", zap.String("node", nodeID), zap.Duration("cost", time.Since(start)))
				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	finalOutput, exists := results[wf.OutputNode]
	if !exists {
		// 降级：使用EntryPoint的输出
		finalOutput = results[wf.OutputNode]
		if finalOutput == nil {
			finalOutput = make(domain.NodeOutput)
		}
	}

	e.logger.Info("workflow completed",
		zap.String("workflow_id", wf.ID),
		zap.Duration("cost", time.Since(start)),
		zap.Int("nodes", len(wf.Nodes)))

	return finalOutput, nil
}

// executeNodeWithRetry 带超时和重试的节点执行
func (e *DAGEngine) executeNodeWithRetry(ctx context.Context, node *domain.Node, inputs domain.NodeOutput) (domain.NodeOutput, error) {
	for attempt := 0; attempt <= node.Retry; attempt++ {
		if attempt > 0 {
			e.logger.Warn("node retry", zap.String("node", node.ID), zap.Int("attempt", attempt))
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, node.Timeout)
		output, err := node.Executor.Execute(timeoutCtx, inputs)
		cancel()

		if err == nil {
			return output, nil
		}
		if attempt == node.Retry {
			return nil, err
		}
		time.Sleep(time.Second * time.Duration(attempt+1)) // 指数退避
	}
	return nil, fmt.Errorf("max retry reached")
}

// mergeInputs 安全合并前置依赖输出（避免 key 冲突）
func (e *DAGEngine) mergeInputs(deps []string, results map[string]domain.NodeOutput, mu *sync.Mutex) domain.NodeOutput {
	mu.Lock()
	defer mu.Unlock()

	merged := make(domain.NodeOutput)
	for _, dep := range deps {
		if out, ok := results[dep]; ok {
			for k, v := range out {
				// 前缀避免冲突：prevNodeKey
				key := fmt.Sprintf("%s_%s", dep, k)
				merged[key] = v
			}
		}
	}
	return merged
}

// validateDAG 验证 DAG 合法性（无环、存在 END 等）
func (e *DAGEngine) validateDAG(wf *domain.Workflow) error {
	if wf.OutputNode == "" {
		return fmt.Errorf("output_node is required")
	}
	if _, hasEnd := wf.Nodes[wf.OutputNode]; !hasEnd {
		return fmt.Errorf("missing end node: %s", wf.OutputNode)
	}
	if wf.EntryPoint == "" {
		return fmt.Errorf("entry_point is required")
	}
	if _, hasEntry := wf.Nodes[wf.EntryPoint]; !hasEntry && wf.EntryPoint != domain.NodeTypeStart.String() {
		return fmt.Errorf("missing entry point: %s", wf.EntryPoint)
	}

	// Kahn算法验证无环 + 连通性
	inDeg := make(map[string]int, len(wf.Nodes))
	for id := range wf.Nodes {
		inDeg[id] = len(wf.Nodes[id].Dependencies)
	}

	queue := make([]string, 0, len(wf.Nodes))
	for id, deg := range inDeg {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	processed := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		processed++

		for id2, n := range wf.Nodes {
			for _, dep := range n.Dependencies {
				if dep == id {
					inDeg[id2]--
					if inDeg[id2] == 0 {
						queue = append(queue, id2)
					}
				}
			}
		}
	}

	if processed != len(wf.Nodes) {
		return fmt.Errorf("invalid dag: cycle detected or unreachable nodes (processed %d/%d)", processed, len(wf.Nodes))
	}

	e.logger.Debug("dag validation passed",
		zap.String("workflow_id", wf.ID),
		zap.Int("nodes", len(wf.Nodes)))

	return nil
}
