package chat

import (
	"context"
	"errors"
	"strings"
	"testing"

	v1 "my-project/api/chat/v1"
	"my-project/internal/agent"
)

type fakeLLM struct {
	receivedMessages []agent.Message
	reply            agent.Message
	err              error
}

func (f *fakeLLM) Chat(ctx context.Context, messages []agent.Message, tools []agent.ToolDefinition) (agent.Message, error) {
	f.receivedMessages = messages
	return f.reply, f.err
}

var _ agent.LLM = (*fakeLLM)(nil)

func TestControllerV1_Chat_Success(t *testing.T) {
	const (
		userPrompt = "你好"
		mockReply  = "你好！我是 AI 助手。"
	)

	fake := &fakeLLM{
		reply: agent.Message{
			Role:    agent.RoleAssistant,
			Content: mockReply,
		},
	}

	registry, err := agent.NewToolRegistry()
	if err != nil {
		t.Fatalf(
			"NewToolRegistry() error = %v",
			err,
		)
	}

	agentInstance, err := agent.NewAgent(fake, registry, agent.Config{MaxTurns: 8})
	if err != nil {
		t.Fatalf(
			"NewAgent() error = %v",
			err,
		)
	}
	controller := NewV1(agentInstance)

	req := &v1.ChatReq{
		Message: userPrompt,
	}

	res, err := controller.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil response, got nil")
	}
	if res.Reply != mockReply {
		t.Errorf("expected reply %q, got %q", mockReply, res.Reply)
	}

	if len(fake.receivedMessages) != 1 {
		t.Fatalf("expected 1 message sent to LLM, got %d", len(fake.receivedMessages))
	}
	received := fake.receivedMessages[0]
	if received.Role != agent.RoleUser {
		t.Errorf("expected role %q, got %q", agent.RoleUser, received.Role)
	}
	if received.Content != userPrompt {
		t.Errorf("expected content %q, got %q", userPrompt, received.Content)
	}
}

func TestControllerV1_Chat_LLMError(t *testing.T) {
	const sensitiveVendorErr = "deepseek upstream error: invalid_request with token sk-secret-test-key"

	fake := &fakeLLM{
		err: errors.New(sensitiveVendorErr),
	}
	registry, err := agent.NewToolRegistry()
	if err != nil {
		t.Fatalf(
			"NewToolRegistry() error = %v",
			err,
		)
	}

	agentInstance, err := agent.NewAgent(fake, registry, agent.Config{MaxTurns: 8})
	if err != nil {
		t.Fatalf(
			"NewAgent() error = %v",
			err,
		)
	}
	controller := NewV1(agentInstance)

	req := &v1.ChatReq{
		Message: "触发异常测试",
	}

	res, err := controller.Chat(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if res != nil {
		t.Errorf("expected nil response on error, got %+v", res)
	}

	if strings.Contains(err.Error(), sensitiveVendorErr) || strings.Contains(err.Error(), "sk-secret-test-key") {
		t.Errorf("sensitive vendor error leaked to client: %v", err)
	}
}
