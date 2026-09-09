package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"my-project/internal/agent"
	"net/http"
	"time"
)

// Config 用于创建Client的配置
type Config struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

// Client 是一个实现了agent.LLM接口的兼容客户端
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewClient 创建一个新的Client实例
func NewClient(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("baseURL is empty")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("apiKey is empty")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("model is empty")
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return &Client{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		httpClient: httpClient,
	}, nil
}

var _ agent.LLM = (*Client)(nil)

// chatMessage 是发送给OpenAI API 的消息结构
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest 是发送给OpenAI API 的请求结构
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

// chatResponse 是OpenAI API 返回的响应结构（目前只关心字段部分）
type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (c *Client) Chat(ctx context.Context, messages []agent.Message) (res agent.Message, err error) {

	apiMessages := make([]chatMessage, len(messages))
	for i, msg := range messages {
		apiMessages[i] = chatMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	reqBody := chatRequest{
		Model:    c.model,
		Messages: apiMessages,
	}

	//序列化请求体
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return agent.Message{}, fmt.Errorf("marshal request body failed: %w", err)
	}

	//创建请求
	url := c.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return agent.Message{}, fmt.Errorf("create request failed: %w", err)
	}

	// 设置请求头
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return agent.Message{}, fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	//检查HTTP状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return agent.Message{}, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// 解析响应
	var apiResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return agent.Message{}, fmt.Errorf("decode response body failed: %w", err)
	}

	//检查返回结果
	if len(apiResp.Choices) == 0 {
		return agent.Message{}, fmt.Errorf("empty choices")
	}

	choice := apiResp.Choices[0]

	//转为内部消息并返回
	return agent.Message{
		Role:    agent.Role(choice.Message.Role),
		Content: choice.Message.Content,
	}, nil
}
