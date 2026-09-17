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

	a := NewAgent(fake, registry)
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

	a := NewAgent(
		llm,
		registry,
	)

	if a.registry != registry {
		t.Fatal("agent registry was not stored")
	}
}
