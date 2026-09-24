package agent

import (
	"context"
	"errors"
	"fmt"
)

func (a *Agent) executeToolCall(ctx context.Context, call ToolCall) (Message, error) {

	if a == nil {
		return Message{}, errors.New("agent: cannot execute tool call with nil agent")
	}

	if a.registry == nil {
		return Message{}, errors.New(
			"agent: tool registry cannot be nil",
		)
	}

	if err := ctx.Err(); err != nil {
		return Message{}, fmt.Errorf(
			"agent: context canceled: %w",
			err,
		)
	}

	result, err := a.registry.Execute(ctx, call)

	if err != nil {
		return Message{}, fmt.Errorf(
			"execute tool call id=%q name=%q: %w",
			call.ID,
			call.Name,
			err,
		)
	}

	return Message{
		Role:       RoleTool,
		Content:    result.Content,
		ToolCallID: call.ID,
	}, nil
}
