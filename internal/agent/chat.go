package agent

import (
	"context"
	"errors"
	"fmt"
)

type Agent struct {
	llm      LLM
	registry *ToolRegistry
	maxTurns int
}

type Config struct {
	MaxTurns int
}

func NewAgent(llm LLM, registry *ToolRegistry, cfg Config) (*Agent, error) {

	if llm == nil {
		return nil, errors.New("agent: llm cannot be nil")
	}

	if registry == nil {
		return nil, errors.New("agent: registry cannot be nil")
	}

	if cfg.MaxTurns <= 0 {
		return nil, fmt.Errorf("agent: Maxturns must be greater than 0: %d", cfg.MaxTurns)
	}

	return &Agent{
		llm:      llm,
		registry: registry,
		maxTurns: cfg.MaxTurns,
	}, nil
}

func (a *Agent) Chat(ctx context.Context, messages []Message) (res Message, err error) {
	return a.llm.Chat(ctx, messages, nil)
}
