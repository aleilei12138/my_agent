package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// toolExecutionFakeTool 是专门用于测试工具执行流程的假 Tool。
//
// 它不会调用真实 API，也不会做真实业务。
// 我们只用它验证：
//
// ToolCall
//
//	↓
//
// ToolRegistry
//
//	↓
//
// Tool.Execute
//
//	↓
//
// ToolResult
//
//	↓
//
// RoleTool Message
type toolExecutionFakeTool struct {
	definition ToolDefinition
	result     ToolResult
	err        error

	receivedArguments json.RawMessage
}

// Definition 实现 Tool 接口。
func (t *toolExecutionFakeTool) Definition() ToolDefinition {
	return t.definition
}

// Execute 实现 Tool 接口。
func (t *toolExecutionFakeTool) Execute(
	ctx context.Context,
	arguments json.RawMessage,
) (ToolResult, error) {
	// 保存一份 arguments 的副本，
	// 方便测试确认 Registry 有没有正确把参数传进来。
	t.receivedArguments = append(
		json.RawMessage(nil),
		arguments...,
	)

	if t.err != nil {
		return ToolResult{}, t.err
	}

	return t.result, nil
}

// toolExecutionFakeLLM 只是为了满足 Agent 对 LLM 的依赖。
//
// executeToolCall 这个测试阶段不会真正调用 LLM。
type toolExecutionFakeLLM struct{}

func (f *toolExecutionFakeLLM) Chat(
	ctx context.Context,
	messages []Message,
	tools []ToolDefinition,
) (Message, error) {
	return Message{}, errors.New(
		"fake LLM should not be called during tool execution test",
	)
}

// TestAgentExecuteToolCall
//
// 验证最核心的 MVP 流程：
//
// ToolCall
//
//	↓
//
// 找到 Tool
//
//	↓
//
// 执行 Tool
//
//	↓
//
// 得到 ToolResult
//
//	↓
//
// 转换成 RoleTool Message
func TestAgentExecuteToolCall(t *testing.T) {
	tool := &toolExecutionFakeTool{
		definition: ToolDefinition{
			Name: "get_weather",
		},
		result: ToolResult{
			Content: "深圳当前28°C",
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
		&toolExecutionFakeLLM{},
		registry,
		Config{MaxTurns: 8},
	)

	call := ToolCall{
		ID:   "call_123",
		Name: "get_weather",
		Arguments: json.RawMessage(
			`{"city":"深圳"}`,
		),
	}

	msg, err := a.executeToolCall(
		context.Background(),
		call,
	)
	if err != nil {
		t.Fatalf(
			"executeToolCall() error = %v",
			err,
		)
	}

	// 1. Tool 执行结果必须转换成 RoleTool。
	if msg.Role != RoleTool {
		t.Fatalf(
			"Role = %q, want %q",
			msg.Role,
			RoleTool,
		)
	}

	// 2. ToolCallID 必须等于原始 ToolCall.ID。
	//
	// 不能使用 Tool Name。
	if msg.ToolCallID != "call_123" {
		t.Fatalf(
			"ToolCallID = %q, want %q",
			msg.ToolCallID,
			"call_123",
		)
	}

	// 3. ToolResult.Content 必须进入 Message.Content。
	if msg.Content != "深圳当前28°C" {
		t.Fatalf(
			"Content = %q, want %q",
			msg.Content,
			"深圳当前28°C",
		)
	}

	// 4. ToolCall.Arguments 必须原样传给 Tool.Execute。
	wantArguments := `{"city":"深圳"}`

	if string(tool.receivedArguments) != wantArguments {
		t.Fatalf(
			"received arguments = %q, want %q",
			string(tool.receivedArguments),
			wantArguments,
		)
	}
}

// TestAgentExecuteToolCallUnknownTool
//
// 验证模型返回一个不存在的工具时，
// Agent 不应该假装执行成功。
func TestAgentExecuteToolCallUnknownTool(t *testing.T) {
	registry, err := NewToolRegistry()
	if err != nil {
		t.Fatalf(
			"NewToolRegistry() error = %v",
			err,
		)
	}

	a, err := NewAgent(
		&toolExecutionFakeLLM{},
		registry,
		Config{MaxTurns: 8},
	)

	call := ToolCall{
		ID:   "call_999",
		Name: "unknown_tool",
	}

	_, err = a.executeToolCall(
		context.Background(),
		call,
	)

	if err == nil {
		t.Fatal(
			"executeToolCall() error = nil, want error",
		)
	}
}

// TestAgentExecuteToolCallToolError
//
// 验证 Tool.Execute 自身失败时，
// error 能够继续向上传递，并保留错误链。
func TestAgentExecuteToolCallToolError(t *testing.T) {
	sentinelErr := errors.New(
		"weather service unavailable",
	)

	tool := &toolExecutionFakeTool{
		definition: ToolDefinition{
			Name: "get_weather",
		},
		err: sentinelErr,
	}

	registry, err := NewToolRegistry(tool)
	if err != nil {
		t.Fatalf(
			"NewToolRegistry() error = %v",
			err,
		)
	}

	a, err := NewAgent(
		&toolExecutionFakeLLM{},
		registry,
		Config{MaxTurns: 8},
	)

	call := ToolCall{
		ID:   "call_123",
		Name: "get_weather",
		Arguments: json.RawMessage(
			`{"city":"深圳"}`,
		),
	}

	_, err = a.executeToolCall(
		context.Background(),
		call,
	)

	if err == nil {
		t.Fatal(
			"executeToolCall() error = nil, want error",
		)
	}

	// 如果 Registry 和 Agent 都使用 %w 包装错误，
	// errors.Is 仍然应该能够找到最底层的 sentinelErr。
	if !errors.Is(err, sentinelErr) {
		t.Fatalf(
			"error = %v, want wrapped error %v",
			err,
			sentinelErr,
		)
	}
}
