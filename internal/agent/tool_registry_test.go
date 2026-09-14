package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeTool struct {
	name        string
	description string
	params      json.RawMessage
	execFunc    func(ctx context.Context, args json.RawMessage) (ToolResult, error)
	called      bool
	lastArgs    json.RawMessage
}

func (f *fakeTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        f.name,
		Description: f.description,
		Parameters:  f.params,
	}
}

func (f *fakeTool) Execute(ctx context.Context, args json.RawMessage) (ToolResult, error) {
	f.called = true
	f.lastArgs = args
	if f.execFunc != nil {
		return f.execFunc(ctx, args)
	}
	return ToolResult{Content: "success", IsError: false}, nil
}

var _ Tool = (*fakeTool)(nil)

// 1. 测试正常注册、参数原样传递与结果正常返回
func TestToolRegistry_Success(t *testing.T) {
	tool := &fakeTool{
		name:        "calculator",
		description: "perform math calculations",
		params:      json.RawMessage(`{"type":"object"}`),
	}

	registry, err := NewToolRegistry(tool)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	rawArgs := json.RawMessage(`{"expression":"1+1"}`)
	call := ToolCall{
		ID:        "call_123",
		Name:      "calculator",
		Arguments: rawArgs,
	}

	res, err := registry.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !tool.called {
		t.Fatal("expected tool to be executed")
	}
	if !bytes.Equal(tool.lastArgs, rawArgs) {
		t.Errorf("expected arguments %s, got %s", rawArgs, tool.lastArgs)
	}
	if res.Content != "success" || res.IsError {
		t.Errorf("unexpected tool result: %+v", res)
	}
}

// 2. 测试注册时的边界校验（nil、空名称、重复名称）
func TestToolRegistry_RegistrationValidation(t *testing.T) {
	t.Run("nil tool", func(t *testing.T) {
		_, err := NewToolRegistry(nil)
		if err == nil {
			t.Fatal("expected error for nil tool, got nil")
		}
	})

	t.Run("empty tool name", func(t *testing.T) {
		tool := &fakeTool{name: "  "}
		_, err := NewToolRegistry(tool)
		if err == nil {
			t.Fatal("expected error for empty tool name, got nil")
		}
	})

	t.Run("duplicate tool name", func(t *testing.T) {
		t1 := &fakeTool{name: "search"}
		t2 := &fakeTool{name: "search"}
		_, err := NewToolRegistry(t1, t2)
		if err == nil {
			t.Fatal("expected error for duplicate tool name, got nil")
		}
	})
}

// 3. 测试未注册工具返回 ErrUnknownTool
func TestToolRegistry_UnknownTool(t *testing.T) {
	registry, err := NewToolRegistry(&fakeTool{name: "echo"})
	if err != nil {
		t.Fatalf("failed to init registry: %v", err)
	}

	call := ToolCall{
		ID:   "call_unknown",
		Name: "file_reader",
	}

	_, err = registry.Execute(context.Background(), call)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrUnknownTool) {
		t.Errorf("expected error matching ErrUnknownTool, got: %v", err)
	}
}

// 4. 测试 ToolCall 基础字段缺失校验
func TestToolRegistry_CallValidation(t *testing.T) {
	registry, err := NewToolRegistry(&fakeTool{name: "echo"})
	if err != nil {
		t.Fatalf("failed to init registry: %v", err)
	}

	t.Run("empty ID", func(t *testing.T) {
		_, err := registry.Execute(context.Background(), ToolCall{Name: "echo"})
		if err == nil {
			t.Fatal("expected error for empty call ID, got nil")
		}
	})

	t.Run("empty Name", func(t *testing.T) {
		_, err := registry.Execute(context.Background(), ToolCall{ID: "call_1"})
		if err == nil {
			t.Fatal("expected error for empty call Name, got nil")
		}
	})
}

// 5. 测试底层 Tool 执行错误正确被 %w 包装
func TestToolRegistry_ExecutionErrorWrapping(t *testing.T) {
	sentinelErr := errors.New("underlying network failure")
	tool := &fakeTool{
		name: "fetch",
		execFunc: func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			return ToolResult{}, sentinelErr
		},
	}

	registry, err := NewToolRegistry(tool)
	if err != nil {
		t.Fatalf("failed to init registry: %v", err)
	}

	call := ToolCall{
		ID:   "call_err",
		Name: "fetch",
	}

	_, err = registry.Execute(context.Background(), call)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinelErr) {
		t.Errorf("expected wrapped error matching sentinelErr, got: %v", err)
	}
}

// 6. 测试已取消的 Context 拒绝执行工具
func TestToolRegistry_CanceledContext(t *testing.T) {
	tool := &fakeTool{name: "long_task"}
	registry, err := NewToolRegistry(tool)
	if err != nil {
		t.Fatalf("failed to init registry: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	call := ToolCall{
		ID:   "call_ctx",
		Name: "long_task",
	}

	_, err = registry.Execute(ctx, call)
	if err == nil {
		t.Fatal("expected error on canceled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}
	if tool.called {
		t.Fatal("tool should not be executed when context is canceled")
	}
}

// 7. 测试 Definitions 按名称稳定升序排序
func TestToolRegistry_DefinitionsSorted(t *testing.T) {
	tools := []Tool{
		&fakeTool{name: "zeta"},
		&fakeTool{name: "alpha"},
		&fakeTool{name: "gamma"},
	}

	registry, err := NewToolRegistry(tools...)
	if err != nil {
		t.Fatalf("failed to init registry: %v", err)
	}

	defs := registry.Definitions()
	if len(defs) != 3 {
		t.Fatalf("expected 3 definitions, got %d", len(defs))
	}

	expectedOrder := []string{"alpha", "gamma", "zeta"}
	for i, name := range expectedOrder {
		if defs[i].Name != name {
			t.Errorf("expected definition at index %d to be %q, got %q", i, name, defs[i].Name)
		}
	}
}
