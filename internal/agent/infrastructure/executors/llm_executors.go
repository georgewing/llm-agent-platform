package executors

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"text/template"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"llm-agent-platform/internal/agent/domain"
	"llm-agent-platform/internal/config"
	"llm-agent-platform/internal/shared/kernel"
)

// LLMExecutor 实现 domain.NodeExecutor 接口，
// 通过 OpenAI-compatible chat/completions API 调用大模型。
type LLMExecutor struct {
	client     *openai.Client
	model      string
	nodeConfig map[string]any // 节点配置（prompt模板、temperature等）
}

// NewLLMExecutor 创建 LLM 执行器，支持任何 OpenAI-compatible 端点（OpenAI / DeepSeek / 本地 vLLM 等）。
func NewLLMExecutor(cfg config.LLMConfig, nodeConfig map[string]any) *LLMExecutor {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4-turbo"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60
	}

	clientCfg := openai.DefaultConfig(cfg.APIKey)
	clientCfg.BaseURL = cfg.BaseURL
	clientCfg.HTTPClient = &http.Client{Timeout: time.Duration(cfg.Timeout) * time.Second}

	return &LLMExecutor{
		client:     openai.NewClientWithConfig(clientCfg),
		model:      cfg.Model,
		nodeConfig: nodeConfig,
	}
}

// Execute 满足 domain.NodeExecutor 接口。
// 输入来自 DAG 引擎合并的上游节点输出；
// 输出固定包含 "response" (完整文本) 和 "model" (实际模型名) 两个 key。
func (e *LLMExecutor) Execute(ctx context.Context, inputs domain.NodeOutput) (domain.NodeOutput, error) {
	// ────────────────── 1. 渲染 Prompt 模板 ──────────────────
	promptTmpl, ok := e.nodeConfig["prompt"].(string)
	if !ok || promptTmpl == "" {
		return nil, kernel.NewValidationError("llm node config must contain 'prompt'")
	}

	tmpl, err := template.New("prompt").Parse(promptTmpl)
	if err != nil {
		return nil, fmt.Errorf("template parsing error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, inputs); err != nil {
		return nil, fmt.Errorf("template execution error: %w", err)
	}
	userPrompt := buf.String()

	// ────────────────── 2. 组装 Messages ──────────────────
	messages := make([]openai.ChatCompletionMessage, 0, 2)

	// 如果节点配置了 system_prompt，则添加 system 角色消息
	if sysPrompt, ok := e.nodeConfig["system_prompt"].(string); ok && sysPrompt != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: sysPrompt,
		})
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userPrompt,
	})

	// ────────────────── 3. 读取可选参数 ──────────────────
	temperature := float32(0.7)
	if t, ok := e.nodeConfig["temperature"].(float64); ok {
		temperature = float32(t)
	}

	maxTokens := 2048
	if m, ok := e.nodeConfig["max_tokens"].(float64); ok && m > 0 {
		maxTokens = int(m)
	}

	// 允许节点级别覆盖模型
	model := e.model
	if m, ok := e.nodeConfig["model"].(string); ok && m != "" {
		model = m
	}

	// ────────────────── 4. 调用 Chat Completions API ──────────────────
	resp, err := e.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("openai chat completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai returned empty choices")
	}

	// ────────────────── 5. 返回标准化输出 ──────────────────
	content := resp.Choices[0].Message.Content

	return domain.NodeOutput{
		"response":          content,
		"model":             resp.Model,
		"prompt_tokens":     resp.Usage.PromptTokens,
		"completion_tokens": resp.Usage.CompletionTokens,
		"total_tokens":      resp.Usage.TotalTokens,
	}, nil
}
