package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

		if response.Role != RoleAssistant {
			return Message{}, fmt.Errorf(
				"agent: unexpected LLM response role %q, want %q",
				response.Role,
				RoleAssistant,
			)
		}

		if len(response.ToolCalls) == 0 {
			return response, nil
		}

		if err := validateAndRecordToolCalls(response.ToolCalls, seenToolCalls); err != nil {
			return Message{}, err
		}

		history = append(history, response)

		for _, call := range response.ToolCalls {

			if err := ctx.Err(); err != nil {
				return Message{}, fmt.Errorf("agent: context canceled before tool execution: % w", err)
			}

			toolMessage, err := a.executeToolCall(ctx, call)
			if err != nil {
				return Message{}, fmt.Errorf("agent: toll call failed: %w", err)
			}

			history = append(history, toolMessage)
		}

	}

	return Message{}, fmt.Errorf("agent: %w", ErrMaxTurns)
}

// 检查重复、空值不仅是当前轮次是否重复。
func validateAndRecordToolCalls(
	calls []ToolCall,
	seen map[string]struct{},
) error {
	currentTurn := make(
		map[string]struct{},
		len(calls),
	)

	for _, call := range calls {
		if strings.TrimSpace(call.ID) == "" {
			return errors.New(
				"agent: tool call ID cannot be empty",
			)
		}

		if _, exists := seen[call.ID]; exists {
			return fmt.Errorf(
				"agent: %w: %q",
				ErrDuplicateToolCallID,
				call.ID,
			)
		}

		if _, exists := currentTurn[call.ID]; exists {
			return fmt.Errorf(
				"agent: %w: %q",
				ErrDuplicateToolCallID,
				call.ID,
			)
		}

		currentTurn[call.ID] = struct{}{}
	}

	for id := range currentTurn {
		seen[id] = struct{}{}
	}

	return nil
}
