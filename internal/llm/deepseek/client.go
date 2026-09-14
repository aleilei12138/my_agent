package deepseek

import (
	"fmt"
	"net/http"

	"context"
	"my-project/internal/agent"

	"strings"
	"time"

	openaisdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type Config struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

type Client struct {
	client *openaisdk.Client
	model  string
}

func NewClient(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("deepseek: apiKey is empty")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("deepseek: model is empty")
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}

	baseURL = strings.TrimRight(baseURL, "/")

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	sdkClient := openaisdk.NewClient(
		option.WithAPIKey(cfg.APIKey),
		option.WithBaseURL(baseURL),
		option.WithHTTPClient(httpClient),
	)

	return &Client{
		client: &sdkClient,
		model:  cfg.Model,
	}, nil
}

var _ agent.LLM = (*Client)(nil)

func toSDKMessages(messages []agent.Message) ([]openaisdk.ChatCompletionMessageParamUnion, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("deepseek: empty messages")
	}

	sdkMessages := make([]openaisdk.ChatCompletionMessageParamUnion, 0, len(messages))
	for i, msg := range messages {
		switch msg.Role {
		case agent.RoleSystem:
			sdkMessages = append(sdkMessages, openaisdk.SystemMessage(msg.Content))
		case agent.RoleUser:
			sdkMessages = append(sdkMessages, openaisdk.UserMessage(msg.Content))
		case agent.RoleAssistant:
			sdkMessages = append(sdkMessages, openaisdk.AssistantMessage(msg.Content))
		default:
			return nil, fmt.Errorf("deepseek: unknown role %q at index %d", msg.Role, i)
		}
	}
	return sdkMessages, nil
}

func (c *Client) Chat(ctx context.Context, messages []agent.Message) (agent.Message, error) {
	sdkMessages, err := toSDKMessages(messages)
	if err != nil {
		return agent.Message{}, fmt.Errorf("deepseek: convert messages to sdk messages failed: %w", err)
	}

	params := openaisdk.ChatCompletionNewParams{
		Model:    c.model,
		Messages: sdkMessages,
	}

	resp, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return agent.Message{}, fmt.Errorf("deepseek:  chat request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return agent.Message{}, fmt.Errorf("deepseek: no choices in response")
	}

	content := resp.Choices[0].Message.Content
	if content == "" {
		return agent.Message{}, fmt.Errorf("deepseek: empty content in response")
	}

	return agent.Message{
		Role:    agent.RoleAssistant,
		Content: content,
	}, nil
}
