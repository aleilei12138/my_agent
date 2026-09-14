package agent

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	// ErrUnknownTool 当请求调用未在注册表中注册的工具时返回
	ErrUnknownTool = errors.New("unknown tool")
)

// ToolRegistry 负责工具的集合管理、定义导出与调用分发
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry 创建并初始化工具注册表，在注册时执行严格校验
func NewToolRegistry(tools ...Tool) (*ToolRegistry, error) {
	registry := &ToolRegistry{
		tools: make(map[string]Tool, len(tools)),
	}

	for _, t := range tools {
		if t == nil {
			return nil, errors.New("tool cannot be nil")
		}

		def := t.Definition()
		name := strings.TrimSpace(def.Name)
		if name == "" {
			return nil, errors.New("tool name cannot be empty")
		}

		if _, exists := registry.tools[name]; exists {
			return nil, fmt.Errorf("duplicate tool name: %q", name)
		}

		registry.tools[name] = t
	}

	return registry, nil
}

// Definitions 返回所有已注册工具的定义，并按工具名称升序稳定排列
func (r *ToolRegistry) Definitions() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}

	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})

	return defs
}

// Execute 调度并执行指定的工具调用，内置前置防御与错误包装
func (r *ToolRegistry) Execute(ctx context.Context, call ToolCall) (ToolResult, error) {
	// 1. 优先检查 context 是否已取消或超时
	if err := ctx.Err(); err != nil {
		return ToolResult{}, err
	}

	// 2. 校验 Call 基础信息
	if strings.TrimSpace(call.ID) == "" {
		return ToolResult{}, errors.New("tool call ID cannot be empty")
	}

	name := strings.TrimSpace(call.Name)
	if name == "" {
		return ToolResult{}, errors.New("tool call Name cannot be empty")
	}

	// 3. 查找工具
	tool, exists := r.tools[name]
	if !exists {
		return ToolResult{}, fmt.Errorf("%w: %s", ErrUnknownTool, name)
	}

	// 4. 原样透传 Arguments 并执行
	res, err := tool.Execute(ctx, call.Arguments)
	if err != nil {
		return ToolResult{}, fmt.Errorf("execute tool %s: %w", name, err)
	}

	return res, nil
}
