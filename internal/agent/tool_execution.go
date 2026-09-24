package agent

import (
	"context"
	"fmt"
)

func (a *Agent) executeToolCall(ctx context.Context, call ToolCall) (Message, error) {

	if err := ctx.Err(); err != nil {
		return Message{}, fmt.Errorf(
			"agent: context canceled: %w",
			err,
		)
	}

	result, err := a.registry.Execute(ctx, call)

	if err != nil {
		return Message{}, fmt.Errorf("execute tool call %q: %w", call.ID, err)
	}

	return Message{
		Role:       RoleTool,
		Content:    result.Content,
		ToolCallID: call.ID,
	}, nil
}
