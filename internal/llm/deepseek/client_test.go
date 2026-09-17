package deepseek

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"my-project/internal/agent"
)

// newTestClient 为每个测试创建一个本地假服务器和 DeepSeek Client。
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClient(Config{
		// 三个斜杠用于验证 TrimRight 是否生效。
		BaseURL:    server.URL + "///",
		APIKey:     "test-api-key",
		Model:      "test-model",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	return client
}

func writeChatResponse(t *testing.T, w http.ResponseWriter, message map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{
		"id":      "test-response",
		"object":  "chat.completion",
		"created": 0,
		"model":   "test-model",
		"choices": []map[string]any{
			{
				"index":         0,
				"message":       message,
				"finish_reason": "stop",
			},
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func TestChatSuccess(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			return
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected path /chat/completions, got %s", r.URL.Path)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-api-key" {
			t.Errorf("expected Authorization header %q, got %q", "Bearer test-api-key", got)
			return
		}
		if contentType := r.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("expected JSON Content-Type, got %q", contentType)
			return
		}

		var requestBody struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Tools []json.RawMessage `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Errorf("decode request body: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		if requestBody.Model != "test-model" {
			t.Errorf("expected model %q, got %q", "test-model", requestBody.Model)
		}
		if len(requestBody.Messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(requestBody.Messages))
		} else {
			if requestBody.Messages[0].Role != "system" {
				t.Errorf("expected first role system, got %q", requestBody.Messages[0].Role)
			}
			if requestBody.Messages[1].Role != "user" {
				t.Errorf("expected second role user, got %q", requestBody.Messages[1].Role)
			}
			if requestBody.Messages[1].Content != "hello" {
				t.Errorf("expected user content hello, got %q", requestBody.Messages[1].Content)
			}
		}
		if len(requestBody.Tools) != 0 {
			t.Errorf("expected no tools for ordinary chat, got %d", len(requestBody.Tools))
		}

		writeChatResponse(t, w, map[string]any{
			"role":    "assistant",
			"content": "hello from deepseek",
		})
	})

	result, err := client.Chat(context.Background(), []agent.Message{
		{Role: agent.RoleSystem, Content: "You are a helpful assistant."},
		{Role: agent.RoleUser, Content: "hello"},
	}, nil)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if result.Role != agent.RoleAssistant {
		t.Errorf("expected role %q, got %q", agent.RoleAssistant, result.Role)
	}
	if result.Content != "hello from deepseek" {
		t.Errorf("expected content %q, got %q", "hello from deepseek", result.Content)
	}
	if len(result.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %#v", result.ToolCalls)
	}
}

func TestChatSendsTools(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Tools []struct {
				Type     string `json:"type"`
				Function struct {
					Name        string         `json:"name"`
					Description string         `json:"description"`
					Parameters  map[string]any `json:"parameters"`
					Strict      *bool          `json:"strict"`
				} `json:"function"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if len(body.Tools) != 1 {
			t.Errorf("expected 1 tool, got %d", len(body.Tools))
		} else {
			tool := body.Tools[0]
			if tool.Type != "function" {
				t.Errorf("expected tool type function, got %q", tool.Type)
			}
			if tool.Function.Name != "get_weather" {
				t.Errorf("expected tool name get_weather, got %q", tool.Function.Name)
			}
			if tool.Function.Description != "Get weather" {
				t.Errorf("unexpected description %q", tool.Function.Description)
			}
			if tool.Function.Strict != nil {
				t.Errorf("strict must be omitted for DeepSeek compatibility, got %v", *tool.Function.Strict)
			}
			if got := tool.Function.Parameters["type"]; got != "object" {
				t.Errorf("expected parameters.type object, got %#v", got)
			}
			properties, ok := tool.Function.Parameters["properties"].(map[string]any)
			if !ok {
				t.Errorf("expected parameters.properties object, got %#v", tool.Function.Parameters["properties"])
			} else if _, ok := properties["city"]; !ok {
				t.Errorf("expected city property, got %#v", properties)
			}
		}

		writeChatResponse(t, w, map[string]any{
			"role":    "assistant",
			"content": "I can help with that.",
		})
	})

	_, err := client.Chat(
		context.Background(),
		[]agent.Message{{Role: agent.RoleUser, Content: "weather?"}},
		[]agent.ToolDefinition{{
			Name:        "get_weather",
			Description: "Get weather",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{"city":{"type":"string"}},
				"required":["city"]
			}`),
		}},
	)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
}

func TestChatInvalidToolParametersDoesNotSendRequest(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not receive request when tool parameters are invalid")
	})

	_, err := client.Chat(
		context.Background(),
		[]agent.Message{{Role: agent.RoleUser, Content: "hello"}},
		[]agent.ToolDefinition{{
			Name:       "broken_tool",
			Parameters: json.RawMessage(`{"type":`),
		}},
	)
	if err == nil {
		t.Fatal("expected invalid parameters error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid tool parameters JSON") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChatParsesToolCalls(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeChatResponse(t, w, map[string]any{
			"role":    "assistant",
			"content": "Calling weather tool",
			"tool_calls": []map[string]any{
				{
					"id":   "call_1",
					"type": "function",
					"function": map[string]any{
						"name":      "get_weather",
						"arguments": `{"city":"Shenzhen"}`,
					},
				},
			},
		})
	})

	result, err := client.Chat(
		context.Background(),
		[]agent.Message{{Role: agent.RoleUser, Content: "weather?"}},
		nil,
	)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if result.Content != "Calling weather tool" {
		t.Errorf("unexpected content %q", result.Content)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	call := result.ToolCalls[0]
	if call.ID != "call_1" || call.Name != "get_weather" {
		t.Errorf("unexpected tool call: %#v", call)
	}
	if string(call.Arguments) != `{"city":"Shenzhen"}` {
		t.Errorf("unexpected arguments %s", call.Arguments)
	}
}

func TestChatAllowsEmptyContentWithToolCalls(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeChatResponse(t, w, map[string]any{
			"role":    "assistant",
			"content": "",
			"tool_calls": []map[string]any{
				{
					"id":   "call_1",
					"type": "function",
					"function": map[string]any{
						"name":      "lookup",
						"arguments": `{}`,
					},
				},
			},
		})
	})

	result, err := client.Chat(
		context.Background(),
		[]agent.Message{{Role: agent.RoleUser, Content: "lookup"}},
		nil,
	)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if result.Content != "" {
		t.Errorf("expected empty content, got %q", result.Content)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %d", len(result.ToolCalls))
	}
}

func TestChatRejectsInvalidToolCallArguments(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeChatResponse(t, w, map[string]any{
			"role":    "assistant",
			"content": "",
			"tool_calls": []map[string]any{
				{
					"id":   "call_1",
					"type": "function",
					"function": map[string]any{
						"name":      "lookup",
						"arguments": `{"broken":`,
					},
				},
			},
		})
	})

	_, err := client.Chat(
		context.Background(),
		[]agent.Message{{Role: agent.RoleUser, Content: "lookup"}},
		nil,
	)
	if err == nil {
		t.Fatal("expected invalid tool call arguments error, got nil")
	}
	if !strings.Contains(err.Error(), "must be valid JSON") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChatRejectsDuplicateToolCallIDs(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeChatResponse(t, w, map[string]any{
			"role":    "assistant",
			"content": "",
			"tool_calls": []map[string]any{
				{
					"id":       "call_same",
					"type":     "function",
					"function": map[string]any{"name": "first", "arguments": `{}`},
				},
				{
					"id":       "call_same",
					"type":     "function",
					"function": map[string]any{"name": "second", "arguments": `{}`},
				},
			},
		})
	})

	_, err := client.Chat(
		context.Background(),
		[]agent.Message{{Role: agent.RoleUser, Content: "hello"}},
		nil,
	)
	if err == nil {
		t.Fatal("expected duplicate tool call ID error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate tool call ID") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChatRejectsUnsupportedToolCallType(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeChatResponse(t, w, map[string]any{
			"role":    "assistant",
			"content": "",
			"tool_calls": []map[string]any{
				{
					"id":   "call_1",
					"type": "custom",
					"custom": map[string]any{
						"name":  "custom_tool",
						"input": "raw input",
					},
				},
			},
		})
	})

	_, err := client.Chat(
		context.Background(),
		[]agent.Message{{Role: agent.RoleUser, Content: "hello"}},
		nil,
	)
	if err == nil {
		t.Fatal("expected unsupported tool call type error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported tool call type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoleToolProducesToolCallID(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role       string `json:"role"`
				Content    string `json:"content"`
				ToolCallID string `json:"tool_call_id"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if len(body.Messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(body.Messages))
		} else {
			toolMessage := body.Messages[1]
			if toolMessage.Role != "tool" {
				t.Errorf("expected tool role, got %q", toolMessage.Role)
			}
			if toolMessage.ToolCallID != "call_123" {
				t.Errorf("expected tool_call_id call_123, got %q", toolMessage.ToolCallID)
			}
			if toolMessage.Content != "sunny" {
				t.Errorf("expected tool content sunny, got %q", toolMessage.Content)
			}
		}

		writeChatResponse(t, w, map[string]any{
			"role":    "assistant",
			"content": "It is sunny.",
		})
	})

	_, err := client.Chat(context.Background(), []agent.Message{
		{Role: agent.RoleUser, Content: "weather?"},
		{Role: agent.RoleTool, Content: "sunny", ToolCallID: "call_123"},
	}, nil)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
}

func TestRoleToolRequiresToolCallID(t *testing.T) {
	_, err := toSDKMessages([]agent.Message{
		{Role: agent.RoleTool, Content: "result"},
	})
	if err == nil {
		t.Fatal("expected missing tool_call_id error, got nil")
	}
	if !strings.Contains(err.Error(), "tool_call_id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAssistantToolCallsAreConvertedBackToSDKMessages(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if len(body.Messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(body.Messages))
		} else {
			assistantMessage := body.Messages[1]
			if assistantMessage.Role != "assistant" {
				t.Errorf("expected assistant role, got %q", assistantMessage.Role)
			}
			if assistantMessage.Content != "I will look it up" {
				t.Errorf("content was not preserved: %q", assistantMessage.Content)
			}
			if len(assistantMessage.ToolCalls) != 1 {
				t.Errorf("expected 1 tool call, got %d", len(assistantMessage.ToolCalls))
			} else {
				call := assistantMessage.ToolCalls[0]
				if call.ID != "call_abc" || call.Type != "function" {
					t.Errorf("unexpected tool call envelope: %#v", call)
				}
				if call.Function.Name != "lookup" || call.Function.Arguments != `{"q":"go"}` {
					t.Errorf("unexpected function call: %#v", call.Function)
				}
			}
		}

		writeChatResponse(t, w, map[string]any{
			"role":    "assistant",
			"content": "ok",
		})
	})

	_, err := client.Chat(context.Background(), []agent.Message{
		{Role: agent.RoleUser, Content: "lookup go"},
		{
			Role:    agent.RoleAssistant,
			Content: "I will look it up",
			ToolCalls: []agent.ToolCall{{
				ID:        "call_abc",
				Name:      "lookup",
				Arguments: json.RawMessage(`{"q":"go"}`),
			}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
}

func TestAssistantToolCallsRejectInvalidArguments(t *testing.T) {
	_, err := toSDKMessages([]agent.Message{{
		Role: agent.RoleAssistant,
		ToolCalls: []agent.ToolCall{{
			ID:        "call_abc",
			Name:      "lookup",
			Arguments: json.RawMessage(`{"q":`),
		}},
	}})
	if err == nil {
		t.Fatal("expected invalid arguments error, got nil")
	}
	if !strings.Contains(err.Error(), "must be valid JSON") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChatServerError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid request", http.StatusBadRequest)
	})

	_, err := client.Chat(context.Background(), []agent.Message{
		{Role: agent.RoleUser, Content: "hello"},
	}, nil)
	if err == nil {
		t.Fatal("expected server error, got nil")
	}
	if !strings.Contains(err.Error(), "chat request failed") {
		t.Errorf("expected wrapped chat error, got %q", err.Error())
	}
}

func TestChatEmptyChoices(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
			"id": "test-response",
			"object": "chat.completion",
			"created": 0,
			"model": "test-model",
			"choices": []
		}`))
		if err != nil {
			t.Errorf("write response: %v", err)
		}
	})

	_, err := client.Chat(context.Background(), []agent.Message{
		{Role: agent.RoleUser, Content: "hello"},
	}, nil)
	if err == nil {
		t.Fatal("expected empty choices error, got nil")
	}
	if !strings.Contains(err.Error(), "no choices") {
		t.Errorf("expected no choices error, got %q", err.Error())
	}
}

func TestChatEmptyContent(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeChatResponse(t, w, map[string]any{
			"role":    "assistant",
			"content": "",
		})
	})

	_, err := client.Chat(context.Background(), []agent.Message{
		{Role: agent.RoleUser, Content: "hello"},
	}, nil)
	if err == nil {
		t.Fatal("expected empty response error, got nil")
	}
	if !strings.Contains(err.Error(), "empty response content and no tool calls") {
		t.Errorf("unexpected error %q", err.Error())
	}
}

func TestChatUnknownRole(t *testing.T) {
	_, err := toSDKMessages([]agent.Message{
		{Role: agent.Role("unknown"), Content: "hello"},
	})
	if err == nil {
		t.Fatal("expected unknown role error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown role") {
		t.Errorf("expected unknown role error, got %q", err.Error())
	}
}

func TestChatEmptyMessages(t *testing.T) {
	_, err := toSDKMessages(nil)
	if err == nil {
		t.Fatal("expected empty messages error, got nil")
	}
	if !strings.Contains(err.Error(), "empty messages") {
		t.Errorf("expected empty messages error, got %q", err.Error())
	}
}

func TestChatCancelledContext(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Chat(ctx, []agent.Message{
		{Role: agent.RoleUser, Content: "hello"},
	}, nil)
	if err == nil {
		t.Fatal("expected cancelled context error, got nil")
	}
}

func TestNewClientValidation(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "empty API key",
			cfg: Config{
				Model: "test-model",
			},
		},
		{
			name: "empty model",
			cfg: Config{
				APIKey: "test-api-key",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewClient(test.cfg)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}
