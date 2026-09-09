package agent

import (
	"context"
	"testing"
)

type FakeLLM struct {
}

func (f *FakeLLM) Chat(ctx context.Context, messages []Message) (res Message, err error) {
	return Message{
		Role:    RoleAssistant,
		Content: "fake response",
	}, nil
}

func TestChat(t *testing.T) {
	llm := &FakeLLM{}

	ctx := context.Background()
	messages := []Message{
		{
			Role:    RoleUser,
			Content: "fake request",
		},
	}
	agent := NewAgent(llm)

	res, err := agent.Chat(ctx, messages)
	if err != nil {
		t.Fatalf("agent.Chat failed: %v", err)
	}

	if res.Content != "fake response" {
		t.Fatalf("expected content 'fake response', got '%s'", res.Content)
	}
}
