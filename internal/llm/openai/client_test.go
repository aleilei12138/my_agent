package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"my-project/internal/agent"
)

// TestChat_Success 测试正常请求的完整流程
func TestChat_Success(t *testing.T) {
	// 用 httptest 创建一个假的 OpenAI 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// 验证请求路径
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected path /v1/chat/completions, got %s", r.URL.Path)
		}

		// 验证 Authorization 头
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-api-key" {
			t.Errorf("expected Authorization 'Bearer test-api-key', got %q", auth)
		}

		// 验证 Content-Type 头
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got %q", ct)
		}

		// 验证请求体
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if req.Model != "gpt-4" {
			t.Errorf("expected model 'gpt-4', got %q", req.Model)
		}

		if len(req.Messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(req.Messages))
		}

		if req.Messages[0].Role != "user" {
			t.Errorf("expected role 'user', got %q", req.Messages[0].Role)
		}

		if req.Messages[0].Content != "hello" {
			t.Errorf("expected content 'hello', got %q", req.Messages[0].Content)
		}

		// 返回模拟的响应
		resp := chatResponse{
			Choices: []struct {
				Message chatMessage `json:"message"`
			}{
				{
					Message: chatMessage{
						Role:    "assistant",
						Content: "hi there",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 创建客户端，指向假服务器
	client, err := NewClient(Config{
		BaseURL:    server.URL,
		APIKey:     "test-api-key",
		Model:      "gpt-4",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// 调用 Chat
	msg, err := client.Chat(context.Background(), []agent.Message{
		{Role: agent.RoleUser, Content: "hello"},
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	// 验证返回值
	if msg.Role != agent.RoleAssistant {
		t.Errorf("expected role %q, got %q", agent.RoleAssistant, msg.Role)
	}

	if msg.Content != "hi there" {
		t.Errorf("expected content 'hi there', got %q", msg.Content)
	}
}

// TestChat_ServerError 测试服务端返回 500 错误
func TestChat_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:    server.URL,
		APIKey:     "test-api-key",
		Model:      "gpt-4",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.Chat(context.Background(), []agent.Message{
		{Role: agent.RoleUser, Content: "hello"},
	})
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}

	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain '500', got %q", err.Error())
	}
}

// TestChat_InvalidJSON 测试服务端返回无效 JSON
func TestChat_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`这不是 JSON`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:    server.URL,
		APIKey:     "test-api-key",
		Model:      "gpt-4",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.Chat(context.Background(), []agent.Message{
		{Role: agent.RoleUser, Content: "hello"},
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// TestChat_EmptyChoices 测试服务端返回空的 choices 数组
func TestChat_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message chatMessage `json:"message"`
			}{},
		})
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:    server.URL,
		APIKey:     "test-api-key",
		Model:      "gpt-4",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.Chat(context.Background(), []agent.Message{
		{Role: agent.RoleUser, Content: "hello"},
	})
	if err == nil {
		t.Fatal("expected error for empty choices, got nil")
	}

	if !strings.Contains(err.Error(), "empty choices") {
		t.Errorf("expected error to contain 'empty choices', got %q", err.Error())
	}
}

// TestChat_ContextCancel 测试 context 取消时请求应该中断
func TestChat_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟慢响应
		select {}
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:    server.URL,
		APIKey:     "test-api-key",
		Model:      "gpt-4",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// 创建一个立即取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, err = client.Chat(ctx, []agent.Message{
		{Role: agent.RoleUser, Content: "hello"},
	})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// TestNewClient_Validation 测试构造函数的参数校验
func TestNewClient_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name:   "empty base URL",
			config: Config{APIKey: "key", Model: "model"},
		},
		{
			name:   "empty API key",
			config: Config{BaseURL: "http://localhost", Model: "model"},
		},
		{
			name:   "empty model",
			config: Config{BaseURL: "http://localhost", APIKey: "key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.config)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
