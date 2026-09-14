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
func newTestClient(
	t *testing.T,
	handler http.HandlerFunc,
) *Client {
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

func TestChatSuccess(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// 不要在 Handler 中使用 t.Fatalf。
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			return
		}

		if r.URL.Path != "/chat/completions" {
			t.Errorf(
				"expected path /chat/completions, got %s",
				r.URL.Path,
			)
			return
		}

		if got := r.Header.Get("Authorization"); got != "Bearer test-api-key" {
			t.Errorf(
				"expected Authorization header %q, got %q",
				"Bearer test-api-key",
				got,
			)
			return
		}

		contentType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf(
				"expected JSON Content-Type, got %q",
				contentType,
			)
			return
		}

		var requestBody struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}

		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Errorf("decode request body: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		if requestBody.Model != "test-model" {
			t.Errorf(
				"expected model %q, got %q",
				"test-model",
				requestBody.Model,
			)
		}

		if len(requestBody.Messages) != 2 {
			t.Errorf(
				"expected 2 messages, got %d",
				len(requestBody.Messages),
			)
		} else {
			if requestBody.Messages[0].Role != "system" {
				t.Errorf(
					"expected first role system, got %q",
					requestBody.Messages[0].Role,
				)
			}

			if requestBody.Messages[1].Role != "user" {
				t.Errorf(
					"expected second role user, got %q",
					requestBody.Messages[1].Role,
				)
			}

			if requestBody.Messages[1].Content != "hello" {
				t.Errorf(
					"expected user content hello, got %q",
					requestBody.Messages[1].Content,
				)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		response := map[string]any{
			"id":      "test-response",
			"object":  "chat.completion",
			"created": 0,
			"model":   "test-model",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "hello from deepseek",
					},
					"finish_reason": "stop",
				},
			},
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("encode response: %v", err)
		}
	})

	result, err := client.Chat(context.Background(), []agent.Message{
		{
			Role:    agent.RoleSystem,
			Content: "You are a helpful assistant.",
		},
		{
			Role:    agent.RoleUser,
			Content: "hello",
		},
	})
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}

	if result.Role != agent.RoleAssistant {
		t.Errorf(
			"expected role %q, got %q",
			agent.RoleAssistant,
			result.Role,
		)
	}

	if result.Content != "hello from deepseek" {
		t.Errorf(
			"expected content %q, got %q",
			"hello from deepseek",
			result.Content,
		)
	}
}

func TestChatServerError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid request", http.StatusBadRequest)
	})

	_, err := client.Chat(context.Background(), []agent.Message{
		{
			Role:    agent.RoleUser,
			Content: "hello",
		},
	})
	if err == nil {
		t.Fatal("expected server error, got nil")
	}

	if !strings.Contains(err.Error(), "chat request failed") {
		t.Errorf(
			"expected wrapped chat error, got %q",
			err.Error(),
		)
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
		{
			Role:    agent.RoleUser,
			Content: "hello",
		},
	})
	if err == nil {
		t.Fatal("expected empty choices error, got nil")
	}

	if !strings.Contains(err.Error(), "no choices") {
		t.Errorf(
			"expected no choices error, got %q",
			err.Error(),
		)
	}
}

func TestChatEmptyContent(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(`{
			"id": "test-response",
			"object": "chat.completion",
			"created": 0,
			"model": "test-model",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": ""
					},
					"finish_reason": "stop"
				}
			]
		}`))
		if err != nil {
			t.Errorf("write response: %v", err)
		}
	})

	_, err := client.Chat(context.Background(), []agent.Message{
		{
			Role:    agent.RoleUser,
			Content: "hello",
		},
	})
	if err == nil {
		t.Fatal("expected empty content error, got nil")
	}

	if !strings.Contains(err.Error(), "empty content") {
		t.Errorf(
			"expected empty content error, got %q",
			err.Error(),
		)
	}
}

func TestChatUnknownRole(t *testing.T) {
	_, err := toSDKMessages([]agent.Message{
		{
			Role:    agent.Role("unknown"),
			Content: "hello",
		},
	})
	if err == nil {
		t.Fatal("expected unknown role error, got nil")
	}

	if !strings.Contains(err.Error(), "unknown role") {
		t.Errorf(
			"expected unknown role error, got %q",
			err.Error(),
		)
	}
}

func TestChatEmptyMessages(t *testing.T) {
	_, err := toSDKMessages(nil)
	if err == nil {
		t.Fatal("expected empty messages error, got nil")
	}

	if !strings.Contains(err.Error(), "empty messages") {
		t.Errorf(
			"expected empty messages error, got %q",
			err.Error(),
		)
	}
}

func TestChatCancelledContext(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not receive an already-cancelled request")
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Chat(ctx, []agent.Message{
		{
			Role:    agent.RoleUser,
			Content: "hello",
		},
	})
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
