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

	history := append(
		[]Message(nil),
		messages...,
	)

	seenToolCalls := make(map[string]struct{})

	definitions := a.registry.Definitions()

	if err := ctx.Err(); err != nil {
		return Message{}, fmt.Errorf(
			"agent: context canceled: %w",
			err,
		)
	}

	for turn := 0; turn < a.maxTurns; turn++ {

		if err := ctx.Err(); err != nil {
			return Message{}, fmt.Errorf(
				"agent: context canceled: %w",
				err,
			)
		}

		response, err := a.llm.Chat(ctx, history, definitions)

		if err != nil {
			return Message{}, fmt.Errorf("agent chat failed: %w", err)
		}

		if len(response.ToolCalls) == 0 {
			return response, nil
		}

		history = append(history, response)

		for _, call := range response.ToolCalls {

			if _, exists := seenToolCalls[call.ID]; exists {
				return Message{}, fmt.Errorf("agent: %w: %s", ErrDuplicateToolCallID, call.ID)
			}

			seenToolCalls[call.ID] = struct{}{}
			toolMessage, err := a.executeToolCall(ctx, call)
			if err != nil {
				return Message{}, fmt.Errorf("agent: tollcall failed: %w", err)
			}

			history = append(history, toolMessage)
		}

	}

	return Message{}, fmt.Errorf("agent: %w", ErrMaxTurns)
}
