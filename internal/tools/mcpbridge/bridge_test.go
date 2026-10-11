package mcpbridge

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeCaller struct {
	called int
	name   string
	args   any
}

func (f *fakeCaller) CallTool(_ context.Context, p *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	f.called++
	f.name = p.Name
	f.args = p.Arguments
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "pong"}}}, nil
}

func TestRemoteTool(t *testing.T) {
	caller := &fakeCaller{}
	tool, err := NewRemoteTool(caller, &mcp.Tool{
		Name:        "echo",
		Description: "Echo text",
		InputSchema: map[string]any{
			"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}},
		},
	}, "mcp_echo")
	if err != nil {
		t.Fatal(err)
	}
	if tool.Definition().Name != "mcp_echo" {
		t.Fatalf("wrong public name: %+v", tool.Definition())
	}
	got, err := tool.Execute(context.Background(), json.RawMessage(`{"text":"hello"}`))
	if err != nil || got.IsError || got.Content != "pong" || caller.called != 1 || caller.name != "echo" {
		t.Fatalf("unexpected result=%+v error=%v caller=%+v", got, err, caller)
	}
}

func TestRemoteToolCancel(t *testing.T) {
	caller := &fakeCaller{}
	tool, err := NewRemoteTool(caller, &mcp.Tool{
		Name: "echo", InputSchema: map[string]any{"type": "object"},
	}, "mcp_echo")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = tool.Execute(ctx, json.RawMessage(`{}`))
	if !errors.Is(err, context.Canceled) || caller.called != 0 {
		t.Fatalf("want canceled and zero calls, got %v calls=%d", err, caller.called)
	}
}
