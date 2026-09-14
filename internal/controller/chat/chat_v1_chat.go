package chat

import (
	"context"
	"errors"

	"github.com/gogf/gf/v2/frame/g"

	v1 "my-project/api/chat/v1"
	"my-project/internal/agent"
)

func (c *ControllerV1) Chat(
	ctx context.Context,
	req *v1.ChatReq,
) (*v1.ChatRes, error) {
	messages := []agent.Message{
		{
			Role:    agent.RoleUser,
			Content: req.Message,
		},
	}

	reply, err := c.agent.Chat(ctx, messages)
	if err != nil {
		g.Log().Errorf(ctx, "agent chat failed: %v", err)
		return nil, errors.New("chat service temporarily unavailable")
	}

	return &v1.ChatRes{
		Reply: reply.Content,
	}, nil
}
