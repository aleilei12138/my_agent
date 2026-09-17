package agent

import "context"

type LLM interface {
	Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (res Message, err error)
}
