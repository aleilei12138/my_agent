package agent

import (
	"context"
	"reflect"
	"testing"
)

type fakeChatLLM struct {
	messages []Message
	tools    []ToolDefinition
	reply    Message
	err      error
}

func (f *fakeChatLLM) Chat(
	ctx context.Context,
	messages []Message,
	tools []ToolDefinition,
) (Message, error) {
	f.messages = messages
	f.tools = tools
	return f.reply, f.err
}

var _ LLM = (*fakeChatLLM)(nil)

func TestAgentChatPassesNilTools(t *testing.T) {
	messages := []Message{{Role: RoleUser, Content: "hello"}}
	expected := Message{Role: RoleAssistant, Content: "hi"}
	fake := &fakeChatLLM{reply: expected}
	registry, err := NewToolRegistry()
	if err != nil {
		t.Fatalf("failed to create tool registry: %v", err)
	}

	a, err := NewAgent(fake, registry, Config{MaxTurns: 8})
	got, err := a.Chat(context.Background(), messages)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("Chat() = %#v, want %#v", got, expected)
	}
	if !reflect.DeepEqual(fake.messages, messages) {
		t.Fatalf("messages = %#v, want %#v", fake.messages, messages)
	}
	if fake.tools != nil {
		t.Fatalf("tools = %#v, want nil", fake.tools)
	}
}

func TestNewAgentStoresRegistry(t *testing.T) {
	llm := &fakeChatLLM{}
	registry, err := NewToolRegistry()
	if err != nil {
		t.Fatalf("failed to create tool registry: %v", err)
	}

	a, err := NewAgent(
		llm,
		registry,
		Config{MaxTurns: 8},
	)

	if a.registry != registry {
		t.Fatal("agent registry was not stored")
	}
}

func TestNewAgentRejectsNilLLM(
	t *testing.T,
) {
	registry, err := NewToolRegistry()
	if err != nil {
		t.Fatalf(
			"NewToolRegistry() error = %v",
			err,
		)
	}

	_, err = NewAgent(
		nil,
		registry,
		Config{
			MaxTurns: 8,
		},
	)

	if err == nil {
		t.Fatal(
			"NewAgent() error = nil, want error",
		)
	}
}

func TestNewAgentRejectsNilRegistry(
	t *testing.T,
) {
	fake := &fakeChatLLM{}

	_, err := NewAgent(
		fake,
		nil,
		Config{
			MaxTurns: 8,
		},
	)

	if err == nil {
		t.Fatal(
			"NewAgent() error = nil, want error",
		)
	}
}

func TestNewAgentRejectsNonPositiveMaxTurns(
	t *testing.T,
) {
	fake := &fakeChatLLM{}

	registry, err := NewToolRegistry()
	if err != nil {
		t.Fatalf(
			"NewToolRegistry() error = %v",
			err,
		)
	}

	tests := []int{
		0,
		-1,
	}

	for _, maxTurns := range tests {
		_, err := NewAgent(
			fake,
			registry,
			Config{
				MaxTurns: maxTurns,
			},
		)

		if err == nil {
			t.Fatalf(
				"NewAgent(MaxTurns=%d) error = nil, want error",
				maxTurns,
			)
		}
	}
}
