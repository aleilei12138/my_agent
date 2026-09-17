package agent

import (
	"context"
)

type Agent struct {
	llm      LLM
	registry *ToolRegistry
}

func NewAgent(llm LLM, registry *ToolRegistry) *Agent {
	return &Agent{
		llm:      llm,
		registry: registry,
	}
}

func (a *Agent) Chat(ctx context.Context, messages []Message) (res Message, err error) {
	return a.llm.Chat(ctx, messages, nil)
}

// func (a *Agent) executeToolCalls(ctx context.Context, calls []ToolCall) ([]Message, error) {
// 	for _, call := range calls {
// 		result, err := a.registry.Execute(ctx, call)
// 		if err != nil {
// 			return nil, err
// 		}

// 	}
// }
