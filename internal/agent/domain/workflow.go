package domain

import (
	"context"
	"time"
)

// NodeType 节点类型（枚举）
type NodeType string

const (
	NodeTypeStart NodeType = "START" // 虚拟起始节点
	NodeTypeLLM   NodeType = "LLM"
	NodeTypeTool  NodeType = "TOOL"
	NodeTypeCode  NodeType = "CODE"
	NodeTypeEnd   NodeType = "END"
)

// String 实现 fmt.Stringer 接口，解决 .String() 调用问题
func (n NodeType) String() string {
	return string(n)
}

// NodeOutput 节点标准化输出
type NodeOutput map[string]any

// Node 工作流节点（领域实体）
type Node struct {
	ID           string
	Type         NodeType
	Name         string         // 可读名称，用于日志
	Dependencies []string       // 前置依赖节点ID
	Config       map[string]any // 节点配置（prompt、tool name、code 等）
	Executor     NodeExecutor   // 执行器接口
	Timeout      time.Duration  // 单个节点超时
	Retry        int            // 重试次数（0=不重试）
}

// NodeExecutor 节点执行器接口
type NodeExecutor interface {
	Execute(ctx context.Context, inputs NodeOutput) (NodeOutput, error)
}

// Workflow 完整工作流（聚合根）
type Workflow struct {
	ID         string
	Name       string
	Version    string
	Nodes      map[string]*Node
	EntryPoint string // 入口节点ID（通常 "start"）
	OutputNode string // 输出节点ID（通常 "end"）
	CreatedAt  time.Time
}

// NewWorkflow 工厂方法
func NewWorkflow(id, name string, nodes map[string]*Node, entry, output string) *Workflow {
	return &Workflow{
		ID:         id,
		Name:       name,
		Version:    "1.0",
		Nodes:      nodes,
		EntryPoint: entry,
		OutputNode: output,
		CreatedAt:  time.Now(),
	}
}
