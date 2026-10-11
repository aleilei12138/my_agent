// Place at internal/tools/mcpbridge/bridge.go in the existing my-project module.
package mcpbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"my-project/internal/agent"
)

// Caller is satisfied by *mcp.ClientSession; small interface makes tests easier.
type Caller interface {
	CallTool(context.Context, *mcp.CallToolParams) (*mcp.CallToolResult, error)
}

type RemoteTool struct {
	caller     Caller
	remoteName string
	definition agent.ToolDefinition
}

func NewRemoteTool(caller Caller, serverTool *mcp.Tool, publicName string) (*RemoteTool, error) {
	if caller == nil || serverTool == nil || serverTool.Name == "" || publicName == "" {
		return nil, errors.New("invalid MCP tool configuration")
	}
	if serverTool.InputSchema == nil {
		return nil, fmt.Errorf("MCP tool %q has no input schema", serverTool.Name)
	}
	params, err := json.Marshal(serverTool.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("marshal schema for MCP tool %q: %w", serverTool.Name, err)
	}
	return &RemoteTool{
		caller:     caller,
		remoteName: serverTool.Name,
		definition: agent.ToolDefinition{
			Name:        publicName,
			Description: serverTool.Description,
			Parameters:  params,
		},
	}, nil
}

func (t *RemoteTool) Definition() agent.ToolDefinition {
	if t == nil {
		return agent.ToolDefinition{}
	}
	def := t.definition
	def.Parameters = append(json.RawMessage(nil), def.Parameters...)
	return def
}

func (t *RemoteTool) Execute(ctx context.Context, raw json.RawMessage) (agent.ToolResult, error) {
	if t == nil || t.caller == nil {
		return agent.ToolResult{}, errors.New("MCP tool is not configured")
	}
	if err := ctx.Err(); err != nil {
		return agent.ToolResult{}, err
	}
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil || args == nil {
		return agent.ToolResult{Content: "arguments must be a JSON object", IsError: true}, nil
	}
	res, err := t.caller.CallTool(ctx, &mcp.CallToolParams{
		Name:      t.remoteName,
		Arguments: args,
	})
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("MCP call %q: %w", t.remoteName, err)
	}
	if res == nil {
		return agent.ToolResult{}, errors.New("MCP returned nil result")
	}
	var parts []string
	for _, content := range res.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	content := strings.Join(parts, "\n")
	if content == "" && res.StructuredContent != nil {
		b, err := json.Marshal(res.StructuredContent)
		if err != nil {
			return agent.ToolResult{}, fmt.Errorf("encode MCP structured result: %w", err)
		}
		content = string(b)
	}
	if content == "" {
		content = "MCP tool returned no text content"
	}
	return agent.ToolResult{Content: content, IsError: res.IsError}, nil
}

var _ agent.Tool = (*RemoteTool)(nil)
