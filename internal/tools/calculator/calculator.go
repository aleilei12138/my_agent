package calculator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"my-project/internal/agent"
)

const schema = `{
	"type": "object",
	"properties": {
		"operation": {
			"type": "string",
			"enum": ["add", "subtract", "multiply", "divide"],
			"description": "Arithmetic operation to perform"
		},
		"a": {
			"type": "number",
			"description": "First operand"
		},
		"b": {
			"type": "number",
			"description": "Second operand"
		}
	},
	"required": ["operation", "a", "b"],
	"additionalProperties": false
}`

type Calculator struct{}

type arguments struct {
	Operation string   `json:"operation"`
	A         *float64 `json:"a"`
	B         *float64 `json:"b"`
}

func New() *Calculator {
	return &Calculator{}
}

func (c *Calculator) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "calculator",
		Description: "Perform basic arithmetic calculations.",
		Parameters:  json.RawMessage(schema),
	}
}

func (a arguments) validationError() string {
	if strings.TrimSpace(a.Operation) == "" {
		return "operation is required"
	}

	if a.A == nil {
		return "a is required"
	}

	if a.B == nil {
		return "b is required"
	}

	switch a.Operation {
	case "add", "subtract", "multiply", "divide":
		return ""

	default:
		return fmt.Sprintf(
			"unsupported operation %q",
			a.Operation,
		)
	}
}

func (c *Calculator) Execute(
	ctx context.Context,
	raw json.RawMessage,
) (agent.ToolResult, error) {
	if err := ctx.Err(); err != nil {
		return agent.ToolResult{}, err
	}

	var args arguments

	if err := json.Unmarshal(raw, &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf(
			"decode calculator arguments: %w",
			err,
		)
	}

	if validationErr := args.validationError(); validationErr != "" {
		return agent.ToolResult{
			Content: validationErr,
			IsError: true,
		}, nil
	}

	var result float64

	switch args.Operation {
	case "add":
		result = *args.A + *args.B

	case "subtract":
		result = *args.A - *args.B

	case "multiply":
		result = *args.A * *args.B

	case "divide":
		if *args.B == 0 {
			return agent.ToolResult{
				Content: "cannot divide by zero",
				IsError: true,
			}, nil
		}

		result = *args.A / *args.B
	}

	return agent.ToolResult{
		Content: fmt.Sprintf("%g", result),
	}, nil
}

var _ agent.Tool = (*Calculator)(nil)
