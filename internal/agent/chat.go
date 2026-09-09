package agent

import (
	"context"
)

type Agent struct {
	llm LLM
}

func NewAgent(llm LLM) *Agent {
	return &Agent{
		llm: llm,
	}
}

func (a *Agent) Chat(ctx context.Context, messages []Message) (res *Message, err error) {
	return a.llm.Chat(ctx, messages)
}
