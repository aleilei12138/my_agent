package agent

import (
	"context"
	"encoding/json"
)

// ToolDefinition 描述工具的元数据及参数规范（面向 LLM）
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCall 描述单次工具调用的请求载荷
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult 描述工具执行完毕后的标准化输出结果
type ToolResult struct {
	Content string `json:"content"`
	IsError bool   `json:"is_error"`
}

// Tool 是所有可调用工具必须实现的领域接口
type Tool interface {
	Definition() ToolDefinition
	Execute(ctx context.Context, arguments json.RawMessage) (ToolResult, error)
}
