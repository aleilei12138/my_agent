package agent

import "context"

type LLM interface {
	Chat(ctx context.Context, messages []Message) (res Message, err error)
}
