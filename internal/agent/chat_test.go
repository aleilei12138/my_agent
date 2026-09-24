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

func TestNewAgentValidation(t *testing.T) {
	registry, err := NewToolRegistry()
	if err != nil {
		t.Fatalf(
			"NewToolRegistry() error = %v",
			err,
		)
	}

	t.Run("nil llm", func(t *testing.T) {
		_, err := NewAgent(
			nil,
			registry,
			Config{MaxTurns: 1},
		)

		if err == nil {
			t.Fatal(
				"NewAgent() error = nil, want error",
			)
		}
	})

	t.Run("nil registry", func(t *testing.T) {
		_, err := NewAgent(
			&loopFakeLLM{},
			nil,
			Config{MaxTurns: 1},
		)

		if err == nil {
			t.Fatal(
				"NewAgent() error = nil, want error",
			)
		}
	})

	t.Run("zero max turns", func(t *testing.T) {
		_, err := NewAgent(
			&loopFakeLLM{},
			registry,
			Config{MaxTurns: 0},
		)

		if err == nil {
			t.Fatal(
				"NewAgent() error = nil, want error",
			)
		}
	})

	t.Run("negative max turns", func(t *testing.T) {
		_, err := NewAgent(
			&loopFakeLLM{},
			registry,
			Config{MaxTurns: -1},
		)

		if err == nil {
			t.Fatal(
				"NewAgent() error = nil, want error",
			)
		}
	})
}

func TestAgentChatRejectsNonAssistantResponse(
	t *testing.T,
) {
	llm := &loopFakeLLM{
		responses: []Message{
			{
				Role:    RoleUser,
				Content: "invalid response",
			},
		},
	}

	registry, err := NewToolRegistry()
	if err != nil {
		t.Fatalf(
			"NewToolRegistry() error = %v",
			err,
		)
	}

	a, err := NewAgent(
		llm,
		registry,
		Config{MaxTurns: 3},
	)
	if err != nil {
		t.Fatalf(
			"NewAgent() error = %v",
			err,
		)
	}

	_, err = a.Chat(
		context.Background(),
		[]Message{
			{
				Role:    RoleUser,
				Content: "hello",
			},
		},
	)

	if err == nil {
		t.Fatal(
			"Chat() error = nil, want error",
		)
	}

	if llm.callCount != 1 {
		t.Fatalf(
			"LLM call count = %d, want 1",
			llm.callCount,
		)
	}
}

func TestAgentChatRejectsEmptyToolCallIDBeforeExecution(
	t *testing.T,
) {
	llm := &loopFakeLLM{
		responses: []Message{
			{
				Role: RoleAssistant,
				ToolCalls: []ToolCall{
					{
						ID:        "call_1",
						Name:      "weather",
						Arguments: json.RawMessage(`{}`),
					},
					{
						ID:        "",
						Name:      "weather",
						Arguments: json.RawMessage(`{}`),
					},
				},
			},
		},
	}

	tool := &loopFakeTool{
		def: ToolDefinition{
			Name: "weather",
		},
	}

	registry, err := NewToolRegistry(tool)
	if err != nil {
		t.Fatalf(
			"NewToolRegistry() error = %v",
			err,
		)
	}

	a, err := NewAgent(
		llm,
		registry,
		Config{MaxTurns: 3},
	)
	if err != nil {
		t.Fatalf(
			"NewAgent() error = %v",
			err,
		)
	}

	_, err = a.Chat(
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
			"Chat() error = nil, want error",
		)
	}

	if tool.executed {
		t.Fatal(
			"tool was executed before the whole ToolCall batch was validated",
		)
	}
}

func TestAgentChatRejectsDuplicateToolCallIDInSameTurnBeforeExecution(
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
					{
						ID:        "same_call",
						Name:      "weather",
						Arguments: json.RawMessage(`{}`),
					},
				},
			},
		},
	}

	tool := &loopFakeTool{
		def: ToolDefinition{
			Name: "weather",
		},
	}

	registry, err := NewToolRegistry(tool)
	if err != nil {
		t.Fatalf(
			"NewToolRegistry() error = %v",
			err,
		)
	}

	a, err := NewAgent(
		llm,
		registry,
		Config{MaxTurns: 3},
	)
	if err != nil {
		t.Fatalf(
			"NewAgent() error = %v",
			err,
		)
	}

	_, err = a.Chat(
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
			"Chat() error = %v, want ErrDuplicateToolCallID",
			err,
		)
	}

	if tool.executed {
		t.Fatal(
			"tool was executed before the whole ToolCall batch was validated",
		)
	}
}

type orderedLoopTool struct {
	def   ToolDefinition
	order *[]string
}

func (t *orderedLoopTool) Definition() ToolDefinition {
	return t.def
}

func (t *orderedLoopTool) Execute(
	ctx context.Context,
	arguments json.RawMessage,
) (ToolResult, error) {
	*t.order = append(
		*t.order,
		t.def.Name,
	)

	return ToolResult{
		Content: t.def.Name + " result",
	}, nil
}

func TestAgentChatExecutesMultipleToolsInOrder(
	t *testing.T,
) {
	llm := &loopFakeLLM{
		responses: []Message{
			{
				Role: RoleAssistant,
				ToolCalls: []ToolCall{
					{
						ID:        "call_a",
						Name:      "tool_a",
						Arguments: json.RawMessage(`{}`),
					},
					{
						ID:        "call_b",
						Name:      "tool_b",
						Arguments: json.RawMessage(`{}`),
					},
				},
			},
			{
				Role:    RoleAssistant,
				Content: "done",
			},
		},
	}

	order := make(
		[]string,
		0,
		2,
	)

	toolA := &orderedLoopTool{
		def: ToolDefinition{
			Name: "tool_a",
		},
		order: &order,
	}

	toolB := &orderedLoopTool{
		def: ToolDefinition{
			Name: "tool_b",
		},
		order: &order,
	}

	registry, err := NewToolRegistry(
		toolA,
		toolB,
	)
	if err != nil {
		t.Fatalf(
			"NewToolRegistry() error = %v",
			err,
		)
	}

	a, err := NewAgent(
		llm,
		registry,
		Config{MaxTurns: 3},
	)
	if err != nil {
		t.Fatalf(
			"NewAgent() error = %v",
			err,
		)
	}

	result, err := a.Chat(
		context.Background(),
		[]Message{
			{
				Role:    RoleUser,
				Content: "run tools",
			},
		},
	)

	if err != nil {
		t.Fatalf(
			"Chat() error = %v",
			err,
		)
	}

	if result.Content != "done" {
		t.Fatalf(
			"result content = %q, want %q",
			result.Content,
			"done",
		)
	}

	if len(order) != 2 ||
		order[0] != "tool_a" ||
		order[1] != "tool_b" {

		t.Fatalf(
			"execution order = %v, want [tool_a tool_b]",
			order,
		)
	}

	secondHistory :=
		llm.receivedMessages[1]

	if len(secondHistory) != 4 {
		t.Fatalf(
			"second history length = %d, want 4",
			len(secondHistory),
		)
	}

	if secondHistory[2].Role != RoleTool ||
		secondHistory[2].ToolCallID != "call_a" {

		t.Fatalf(
			"message[2] = %+v, want tool result for call_a",
			secondHistory[2],
		)
	}

	if secondHistory[3].Role != RoleTool ||
		secondHistory[3].ToolCallID != "call_b" {

		t.Fatalf(
			"message[3] = %+v, want tool result for call_b",
			secondHistory[3],
		)
	}
}
