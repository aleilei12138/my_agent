package agent

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

var (
	// ErrUnknownTool 当请求调用未在注册表中注册的工具时返回
	ErrUnknownTool = errors.New("unknown tool")
)

// ToolRegistry 负责工具的集合管理、定义导出与调用分发
type ToolRegistry struct {
	tools       map[string]Tool
	definitions map[string]ToolDefinition
}

// NewToolRegistry 创建并初始化工具注册表，在注册时执行严格校验
func NewToolRegistry(tools ...Tool) (*ToolRegistry, error) {
	registry := &ToolRegistry{
		tools:       make(map[string]Tool, len(tools)),
		definitions: make(map[string]ToolDefinition, len(tools)),
	}

	for _, t := range tools {
		if isNilTool(t) {
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

		// 保存规范化后的定义快照，避免后续重复调用 Definition 导致名称变化。
		def.Name = name
		def.Parameters = append([]byte(nil), def.Parameters...)
		registry.tools[name] = t
		registry.definitions[name] = def
	}

	return registry, nil
}

// isNilTool 同时识别 nil 接口和内部保存了 nil 指针等值的接口。
func isNilTool(tool Tool) bool {
	if tool == nil {
		return true
	}

	value := reflect.ValueOf(tool)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// Definitions 返回所有已注册工具的定义，并按工具名称升序稳定排列
func (r *ToolRegistry) Definitions() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(r.definitions))
	for _, def := range r.definitions {
		// 返回副本，避免调用者修改注册表内部保存的参数定义。
		def.Parameters = append([]byte(nil), def.Parameters...)
		defs = append(defs, def)
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
