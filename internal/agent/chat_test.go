package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type loopFakeLLM struct {
	responses []Message

	callCount int

	receivedMessages [][]Message
	receivedTools    [][]ToolDefinition
}

func (f *loopFakeLLM) Chat(
	ctx context.Context,
	messages []Message,
	tools []ToolDefinition,
) (Message, error) {
	f.callCount++

	f.receivedMessages = append(
		f.receivedMessages,
		append([]Message(nil), messages...),
	)

	f.receivedTools = append(
		f.receivedTools,
		append([]ToolDefinition(nil), tools...),
	)

	index := f.callCount - 1

	if index >= len(f.responses) {
		return Message{}, errors.New(
			"loop fake llm: no more responses",
		)
	}

	return f.responses[index], nil
}

var _ LLM = (*loopFakeLLM)(nil)

type loopFakeTool struct {
	def ToolDefinition

	executed bool

	receivedArguments json.RawMessage
}

func (f *loopFakeTool) Definition() ToolDefinition {
	return f.def
}

func (f *loopFakeTool) Execute(
	ctx context.Context,
	arguments json.RawMessage,
) (ToolResult, error) {

	f.executed = true

	f.receivedArguments = append(
		json.RawMessage(nil),
		arguments...,
	)

	return ToolResult{
		Content: "fake result",
	}, nil
}

var _ Tool = (*loopFakeTool)(nil)

func TestAgentChatFinalAnswerOnly(t *testing.T) {
	llm := &loopFakeLLM{
		responses: []Message{
			{
				Role:    RoleAssistant,
				Content: "hello",
			},
		},
	}

	registry, err := NewToolRegistry()
	if err != nil {
		t.Fatalf(
			"create registry failed: %v",
			err,
		)
	}

	agent, err := NewAgent(
		llm,
		registry,
		Config{
			MaxTurns: 3,
		},
	)
	if err != nil {
		t.Fatalf(
			"create agent failed: %v",
			err,
		)
	}

	result, err := agent.Chat(
		context.Background(),
		[]Message{
			{
				Role:    RoleUser,
				Content: "hello",
			},
		},
	)

	if err != nil {
		t.Fatalf(
			"Chat failed: %v",
			err,
		)
	}

	if result.Content != "hello" {
		t.Fatalf(
			"unexpected result: %v",
			result.Content,
		)
	}

	if llm.callCount != 1 {
		t.Fatalf(
			"LLM called %d times, want 1",
			llm.callCount,
		)
	}
}

func TestAgentChatPassesToolDefinitions(t *testing.T) {
	llm := &loopFakeLLM{
		responses: []Message{
			{
				Role:    RoleAssistant,
				Content: "ok",
			},
		},
	}

	weatherTool := &loopFakeTool{
		def: ToolDefinition{
			Name:        "weather",
			Description: "get weather",
		},
	}

	registry, err := NewToolRegistry(
		weatherTool,
	)
	if err != nil {
		t.Fatalf(
			"registry error: %v",
			err,
		)
	}

	agent, err := NewAgent(
		llm,
		registry,
		Config{
			MaxTurns: 3,
		},
	)

	if err != nil {
		t.Fatalf(
			"agent error: %v",
			err,
		)
	}

	_, err = agent.Chat(
		context.Background(),
		[]Message{
			{
				Role:    RoleUser,
				Content: "weather",
			},
		},
	)

	if err != nil {
		t.Fatalf(
			"Chat error: %v",
			err,
		)
	}

	if llm.callCount != 1 {
		t.Fatalf(
			"LLM call=%d want 1",
			llm.callCount,
		)
	}

	if len(llm.receivedTools) != 1 {
		t.Fatalf(
			"tools calls=%d want 1",
			len(llm.receivedTools),
		)
	}

	if len(llm.receivedTools[0]) != 1 {
		t.Fatalf(
			"tools length=%d want 1",
			len(llm.receivedTools[0]),
		)
	}

	if llm.receivedTools[0][0].Name != "weather" {
		t.Fatalf(
			"unexpected tool: %s",
			llm.receivedTools[0][0].Name,
		)
	}
}

func TestAgentChatMaxTurns(t *testing.T) {
	llm := &loopFakeLLM{
		responses: []Message{
			{
				Role: RoleAssistant,
				ToolCalls: []ToolCall{
					{
						ID:        "call1",
						Name:      "weather",
						Arguments: json.RawMessage(`{}`),
					},
				},
			},
			{
				Role: RoleAssistant,
				ToolCalls: []ToolCall{
					{
						ID:        "call2",
						Name:      "weather",
						Arguments: json.RawMessage(`{}`),
					},
				},
			},
		},
	}

	weatherTool := &loopFakeTool{
		def: ToolDefinition{
			Name:        "weather",
			Description: "get weather",
		},
	}

	registry, err := NewToolRegistry(
		weatherTool,
	)

	if err != nil {
		t.Fatalf(
			"registry error: %v",
			err,
		)
	}

	agent, err := NewAgent(
		llm,
		registry,
		Config{
			MaxTurns: 2,
		},
	)

	if err != nil {
		t.Fatalf(
			"agent error: %v",
			err,
		)
	}

	_, err = agent.Chat(
		context.Background(),
		[]Message{
			{
				Role:    RoleUser,
				Content: "test",
			},
		},
	)

	if err == nil {
		t.Fatal(
			"expected max turn error",
		)
	}

	if llm.callCount != 2 {
		t.Fatalf(
			"LLM calls=%d want 2",
			llm.callCount,
		)
	}
}

func TestAgentChatDoesNotModifyMessages(t *testing.T) {
	llm := &loopFakeLLM{
		responses: []Message{
			{
				Role:    RoleAssistant,
				Content: "ok",
			},
		},
	}

	registry, err := NewToolRegistry()
	if err != nil {
		t.Fatalf(
			"registry error: %v",
			err,
		)
	}

	agent, err := NewAgent(
		llm,
		registry,
		Config{
			MaxTurns: 2,
		},
	)

	if err != nil {
		t.Fatalf(
			"agent error: %v",
			err,
		)
	}

	messages := []Message{
		{
			Role:    RoleUser,
			Content: "hello",
		},
	}

	originalLen := len(messages)

	_, err = agent.Chat(
		context.Background(),
		messages,
	)

	if err != nil {
		t.Fatalf(
			"Chat error: %v",
			err,
		)
	}

	if len(messages) != originalLen {
		t.Fatal(
			"input messages modified",
		)
	}
}

func TestAgentChatExecutesToolAndContinues(
	t *testing.T,
) {

	llm := &loopFakeLLM{

		responses: []Message{

			// 第一次 LLM 返回工具调用
			{
				Role: RoleAssistant,

				ToolCalls: []ToolCall{

					{
						ID:   "call_001",
						Name: "weather",
						Arguments: json.RawMessage(
							`{"city":"beijing"}`,
						),
					},
				},
			},

			// 第二次 LLM 返回最终答案
			{
				Role: RoleAssistant,

				Content: "北京天气很好",
			},
		},
	}

	weatherTool := &loopFakeTool{

		def: ToolDefinition{
			Name: "weather",

			Description: "get weather",
		},
	}

	registry, err := NewToolRegistry(
		weatherTool,
	)

	if err != nil {
		t.Fatalf(
			"create registry failed: %v",
			err,
		)
	}

	agent, err := NewAgent(
		llm,
		registry,
		Config{
			MaxTurns: 3,
		},
	)

	if err != nil {
		t.Fatalf(
			"create agent failed: %v",
			err,
		)
	}

	result, err := agent.Chat(
		context.Background(),

		[]Message{

			{
				Role: RoleUser,

				Content: "北京天气怎么样？",
			},
		},
	)

	if err != nil {

		t.Fatalf(
			"Chat failed: %v",
			err,
		)
	}

	// 最终回答检查

	if result.Content != "北京天气很好" {

		t.Fatalf(
			"unexpected result: %s",
			result.Content,
		)
	}

	// LLM应该调用两次

	if llm.callCount != 2 {

		t.Fatalf(
			"LLM calls=%d want 2",
			llm.callCount,
		)
	}

	// 工具必须执行

	if !weatherTool.executed {

		t.Fatal(
			"tool was not executed",
		)
	}

	// 检查工具参数

	if string(weatherTool.receivedArguments) !=
		`{"city":"beijing"}` {

		t.Fatalf(
			"unexpected arguments: %s",
			weatherTool.receivedArguments,
		)
	}

	// 检查第二次 LLM 收到的历史

	if len(llm.receivedMessages) != 2 {

		t.Fatalf(
			"received message history count=%d want 2",
			len(llm.receivedMessages),
		)
	}

	secondHistory :=
		llm.receivedMessages[1]

	// 第二轮应该有：

	// user
	// assistant(tool call)
	// tool(result)

	if len(secondHistory) != 3 {

		t.Fatalf(
			"second history length=%d want 3",
			len(secondHistory),
		)
	}

	if secondHistory[0].Role != RoleUser {

		t.Fatalf(
			"message[0] role=%s want user",
			secondHistory[0].Role,
		)
	}

	if secondHistory[1].Role != RoleAssistant {

		t.Fatalf(
			"message[1] role=%s want assistant",
			secondHistory[1].Role,
		)
	}

	if len(secondHistory[1].ToolCalls) != 1 {

		t.Fatalf(
			"assistant tool calls=%d want 1",
			len(secondHistory[1].ToolCalls),
		)
	}

	if secondHistory[2].Role != RoleTool {

		t.Fatalf(
			"message[2] role=%s want tool",
			secondHistory[2].Role,
		)
	}

	if secondHistory[2].ToolCallID != "call_001" {

		t.Fatalf(
			"tool call id=%s want call_001",
			secondHistory[2].ToolCallID,
		)
	}
}

func TestAgentChatReturnsMaxTurnsError(
	t *testing.T,
) {
	llm := &loopFakeLLM{
		responses: []Message{
			{
				Role: RoleAssistant,
				ToolCalls: []ToolCall{
					{
						ID:        "call1",
						Name:      "weather",
						Arguments: json.RawMessage(`{}`),
					},
				},
			},
		},
	}

	weatherTool := &loopFakeTool{
		def: ToolDefinition{
			Name: "weather",
		},
	}

	registry, err := NewToolRegistry(
		weatherTool,
	)

	if err != nil {
		t.Fatalf(
			"registry error:%v",
			err,
		)
	}

	agent, err := NewAgent(
		llm,
		registry,
		Config{
			MaxTurns: 1,
		},
	)

	if err != nil {
		t.Fatalf(
			"agent error:%v",
			err,
		)
	}

	_, err = agent.Chat(
		context.Background(),
		[]Message{
			{
				Role:    RoleUser,
				Content: "test",
			},
		},
	)

	if !errors.Is(
		err,
		ErrMaxTurns,
	) {
		t.Fatalf(
			"expected ErrMaxTurns, got %v",
			err,
		)
	}
}

func TestAgentChatRejectsDuplicateToolCallID(
	t *testing.T,
) {

	llm := &loopFakeLLM{
		responses: []Message{

			{
				Role: RoleAssistant,

				ToolCalls: []ToolCall{
					{
						ID:        "same_call",
						Name:      "weather",
						Arguments: json.RawMessage(`{}`),
					},
				},
			},

			{
				Role: RoleAssistant,

				ToolCalls: []ToolCall{
					{
						ID:        "same_call",
						Name:      "weather",
						Arguments: json.RawMessage(`{}`),
					},
				},
			},
		},
	}

	weatherTool := &loopFakeTool{
		def: ToolDefinition{
			Name: "weather",
		},
	}

	registry, err := NewToolRegistry(
		weatherTool,
	)

	if err != nil {
		t.Fatalf(
			"registry error:%v",
			err,
		)
	}

	agent, err := NewAgent(
		llm,
		registry,
		Config{
			MaxTurns: 3,
		},
	)

	if err != nil {
		t.Fatalf(
			"agent error:%v",
			err,
		)
	}

	_, err = agent.Chat(
		context.Background(),
		[]Message{
			{
				Role:    RoleUser,
				Content: "test",
			},
		},
	)

	if !errors.Is(
		err,
		ErrDuplicateToolCallID,
	) {

		t.Fatalf(
			"expected ErrDuplicateToolCallID, got %v",
			err,
		)
	}

	if llm.callCount != 2 {

		t.Fatalf(
			"LLM calls=%d want2",
			llm.callCount,
		)
	}
}

func TestAgentChatContextAlreadyCancelled(
	t *testing.T,
) {

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	cancel()

	llm := &loopFakeLLM{
		responses: []Message{
			{
				Role:    RoleAssistant,
				Content: "hello",
			},
		},
	}

	registry, _ :=
		NewToolRegistry()

	agent, _ :=
		NewAgent(
			llm,
			registry,
			Config{
				MaxTurns: 3,
			},
		)

	_, err :=
		agent.Chat(
			ctx,
			[]Message{
				{
					Role:    RoleUser,
					Content: "hi",
				},
			},
		)

	if err == nil {
		t.Fatal(
			"expected context error",
		)
	}

	if llm.callCount != 0 {
		t.Fatalf(
			"LLM calls=%d want 0",
			llm.callCount,
		)
	}
}

type cancelTool struct {
	def ToolDefinition

	cancel context.CancelFunc
}

func (t *cancelTool) Definition() ToolDefinition {
	return t.def
}

func (t *cancelTool) Execute(
	ctx context.Context,
	arguments json.RawMessage,
) (ToolResult, error) {

	t.cancel()

	return ToolResult{}, ctx.Err()
}

func TestAgentChatToolContextCancelled(
	t *testing.T,
) {

	ctx, cancel :=
		context.WithCancel(
			context.Background(),
		)

	llm := &loopFakeLLM{

		responses: []Message{

			{
				Role: RoleAssistant,

				ToolCalls: []ToolCall{

					{
						ID: "call_cancel",

						Name: "cancel_tool",

						Arguments: json.RawMessage(`{}`),
					},
				},
			},
		},
	}

	cancelTool := &cancelTool{

		def: ToolDefinition{
			Name: "cancel_tool",
		},

		cancel: cancel,
	}

	registry, err :=
		NewToolRegistry(
			cancelTool,
		)

	if err != nil {
		t.Fatalf(
			"registry error:%v",
			err,
		)
	}

	agent, err :=
		NewAgent(
			llm,
			registry,
			Config{
				MaxTurns: 3,
			},
		)

	if err != nil {
		t.Fatalf(
			"agent error:%v",
			err,
		)
	}

	_, err =
		agent.Chat(
			ctx,
			[]Message{

				{
					Role: RoleUser,

					Content: "test",
				},
			},
		)

	if err == nil {
		t.Fatal(
			"expected context cancellation error",
		)
	}

	if !errors.Is(
		err,
		context.Canceled,
	) {

		t.Fatalf(
			"expected context.Canceled got %v",
			err,
		)
	}
}

type errorResultTool struct {
	def ToolDefinition
}

func (t *errorResultTool) Definition() ToolDefinition {

	return t.def
}

func (t *errorResultTool) Execute(
	ctx context.Context,
	arguments json.RawMessage,
) (ToolResult, error) {

	return ToolResult{

		Content: "weather api unavailable",

		IsError: true,
	}, nil
}

func TestAgentChatContinuesAfterToolResultError(
	t *testing.T,
) {

	llm := &loopFakeLLM{

		responses: []Message{

			{
				Role: RoleAssistant,

				ToolCalls: []ToolCall{

					{
						ID: "call_error",

						Name: "weather",

						Arguments: json.RawMessage(`{}`),
					},
				},
			},

			{
				Role: RoleAssistant,

				Content: "天气服务暂时不可用",
			},
		},
	}

	tool := &errorResultTool{

		def: ToolDefinition{
			Name: "weather",
		},
	}

	registry, err :=
		NewToolRegistry(
			tool,
		)

	if err != nil {
		t.Fatalf(
			"registry error:%v",
			err,
		)
	}

	agent, err :=
		NewAgent(
			llm,
			registry,
			Config{
				MaxTurns: 3,
			},
		)

	if err != nil {
		t.Fatalf(
			"agent error:%v",
			err,
		)
	}

	result, err :=
		agent.Chat(
			context.Background(),

			[]Message{

				{
					Role: RoleUser,

					Content: "北京天气怎么样",
				},
			},
		)

	if err != nil {
		t.Fatalf(
			"Chat error:%v",
			err,
		)
	}

	if result.Content !=
		"天气服务暂时不可用" {

		t.Fatalf(
			"unexpected result:%s",
			result.Content,
		)
	}

	if llm.callCount != 2 {

		t.Fatalf(
			"LLM calls=%d want2",
			llm.callCount,
		)
	}

	secondHistory :=
		llm.receivedMessages[1]

	if len(secondHistory) != 3 {

		t.Fatalf(
			"history length=%d want3",
			len(secondHistory),
		)
	}

	if secondHistory[2].Role != RoleTool {

		t.Fatalf(
			"expected tool message",
		)
	}
}
