package calculator

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestCalculatorDefinition(t *testing.T) {
	tool := New()

	def := tool.Definition()

	if def.Name != "calculator" {
		t.Fatalf(
			"Definition().Name = %q, want %q",
			def.Name,
			"calculator",
		)
	}

	if len(def.Parameters) == 0 {
		t.Fatal(
			"Definition().Parameters is empty",
		)
	}

	if !json.Valid(def.Parameters) {
		t.Fatal(
			"Definition().Parameters is not valid JSON",
		)
	}
}

func TestCalculatorExecute(t *testing.T) {
	tests := []struct {
		name      string
		arguments string
		want      string
	}{
		{
			name: "add",
			arguments: `{
				"operation": "add",
				"a": 10,
				"b": 20
			}`,
			want: "30",
		},
		{
			name: "subtract",
			arguments: `{
				"operation": "subtract",
				"a": 10,
				"b": 3
			}`,
			want: "7",
		},
		{
			name: "multiply",
			arguments: `{
				"operation": "multiply",
				"a": 6,
				"b": 7
			}`,
			want: "42",
		},
		{
			name: "divide",
			arguments: `{
				"operation": "divide",
				"a": 20,
				"b": 4
			}`,
			want: "5",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				tool := New()

				result, err := tool.Execute(
					context.Background(),
					json.RawMessage(
						tt.arguments,
					),
				)

				if err != nil {
					t.Fatalf(
						"Execute() error = %v",
						err,
					)
				}

				if result.IsError {
					t.Fatal(
						"Execute() IsError = true, want false",
					)
				}

				if result.Content != tt.want {
					t.Fatalf(
						"Execute() Content = %q, want %q",
						result.Content,
						tt.want,
					)
				}
			},
		)
	}
}

func TestCalculatorDivideByZero(t *testing.T) {
	tool := New()

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{
			"operation": "divide",
			"a": 10,
			"b": 0
		}`),
	)

	if err != nil {
		t.Fatalf(
			"Execute() error = %v",
			err,
		)
	}

	if !result.IsError {
		t.Fatal(
			"Execute() IsError = false, want true",
		)
	}

	if result.Content == "" {
		t.Fatal(
			"Execute() Content is empty",
		)
	}
}

func TestCalculatorUnsupportedOperation(
	t *testing.T,
) {
	tool := New()

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{
			"operation": "power",
			"a": 2,
			"b": 8
		}`),
	)

	if err != nil {
		t.Fatalf(
			"Execute() error = %v",
			err,
		)
	}

	if !result.IsError {
		t.Fatal(
			"Execute() IsError = false, want true",
		)
	}
}

func TestCalculatorInvalidJSON(t *testing.T) {
	tool := New()

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{"operation":`),
	)

	if err == nil {
		t.Fatal(
			"Execute() error = nil, want error",
		)
	}
}

func TestCalculatorContextCancelled(
	t *testing.T,
) {
	ctx, cancel :=
		context.WithCancel(
			context.Background(),
		)

	cancel()

	tool := New()

	_, err := tool.Execute(
		ctx,
		json.RawMessage(`{
			"operation": "add",
			"a": 1,
			"b": 2
		}`),
	)

	if !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatalf(
			"Execute() error = %v, want context.Canceled",
			err,
		)
	}
}
