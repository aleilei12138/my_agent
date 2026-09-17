package deepseek

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	openaisdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"

	"my-project/internal/agent"
)

const defaultBaseURL = "https://api.deepseek.com"

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
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("deepseek: apiKey is empty")
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		return nil, fmt.Errorf("deepseek: model is empty")
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	sdkClient := openaisdk.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
		option.WithHTTPClient(httpClient),
	)

	return &Client{
		client: &sdkClient,
		model:  model,
	}, nil
}

var _ agent.LLM = (*Client)(nil)

// toSDKTools converts Agent-owned tool definitions into OpenAI-compatible
// Chat Completions function tools. It performs format validation only and does
// not execute tools.
func toSDKTools(
	definitions []agent.ToolDefinition,
) ([]openaisdk.ChatCompletionToolUnionParam, error) {

	if len(definitions) == 0 {
		return nil, nil
	}

	tools := make([]openaisdk.ChatCompletionToolUnionParam, 0, len(definitions))
	for i, definition := range definitions {
		name := strings.TrimSpace(definition.Name)
		if name == "" {
			return nil, fmt.Errorf("deepseek: tool name cannot be empty at index %d", i)
		}

		function := shared.FunctionDefinitionParam{
			Name: name,
		}
		if definition.Description != "" {
			function.Description = openaisdk.String(definition.Description)
		}

		if len(definition.Parameters) > 0 {
			var parameters shared.FunctionParameters
			if err := json.Unmarshal(definition.Parameters, &parameters); err != nil {
				return nil, fmt.Errorf(
					"deepseek: invalid tool parameters JSON for %q: %w",
					name,
					err,
				)
			}
			function.Parameters = parameters
		}

		tools = append(tools, openaisdk.ChatCompletionFunctionTool(function))
	}

	return tools, nil
}

// toSDKMessages converts the Agent domain messages into SDK request messages.
// Assistant tool calls and tool results are preserved so a future Agent Loop
// can append them back to the conversation history without losing protocol data.
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
			assistantMessage := openaisdk.AssistantMessage(msg.Content)
			if len(msg.ToolCalls) > 0 {
				toolCalls := make(
					[]openaisdk.ChatCompletionMessageToolCallUnionParam,
					0,
					len(msg.ToolCalls),
				)
				for callIndex, call := range msg.ToolCalls {
					id := strings.TrimSpace(call.ID)
					if id == "" {
						return nil, fmt.Errorf(
							"deepseek: assistant tool call ID cannot be empty at message %d call %d",
							i,
							callIndex,
						)
					}

					name := strings.TrimSpace(call.Name)
					if name == "" {
						return nil, fmt.Errorf(
							"deepseek: assistant tool call name cannot be empty at message %d call %d",
							i,
							callIndex,
						)
					}

					if !json.Valid(call.Arguments) {
						return nil, fmt.Errorf(
							"deepseek: assistant tool call arguments for %q must be valid JSON",
							name,
						)
					}

					toolCalls = append(
						toolCalls,
						openaisdk.ChatCompletionMessageToolCallUnionParam{
							OfFunction: &openaisdk.ChatCompletionMessageFunctionToolCallParam{
								ID: id,
								Function: openaisdk.ChatCompletionMessageFunctionToolCallFunctionParam{
									Name:      name,
									Arguments: string(call.Arguments),
								},
							},
						},
					)
				}

				if assistantMessage.OfAssistant == nil {
					return nil, fmt.Errorf("deepseek: SDK assistant message conversion failed")
				}
				assistantMessage.OfAssistant.ToolCalls = toolCalls
			}
			sdkMessages = append(sdkMessages, assistantMessage)

		case agent.RoleTool:
			toolCallID := strings.TrimSpace(msg.ToolCallID)
			if toolCallID == "" {
				return nil, fmt.Errorf("deepseek: tool message must have a non-empty tool_call_id at index %d", i)
			}
			sdkMessages = append(
				sdkMessages,
				openaisdk.ToolMessage(msg.Content, toolCallID),
			)

		default:
			return nil, fmt.Errorf("deepseek: unknown role %q at index %d", msg.Role, i)
		}
	}

	return sdkMessages, nil
}

func (c *Client) Chat(
	ctx context.Context,
	messages []agent.Message,
	tools []agent.ToolDefinition,
) (agent.Message, error) {
	sdkMessages, err := toSDKMessages(messages)
	if err != nil {
		return agent.Message{}, fmt.Errorf("deepseek: convert messages to sdk messages failed: %w", err)
	}

	sdkTools, err := toSDKTools(tools)
	if err != nil {
		return agent.Message{}, fmt.Errorf("deepseek: convert tools to sdk tools failed: %w", err)
	}

	params := openaisdk.ChatCompletionNewParams{
		Model:    c.model,
		Messages: sdkMessages,
	}
	if len(sdkTools) > 0 {
		params.Tools = sdkTools
	}

	resp, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return agent.Message{}, fmt.Errorf("deepseek: chat request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return agent.Message{}, fmt.Errorf("deepseek: no choices in response")
	}

	sdkMessage := resp.Choices[0].Message
	var toolCalls []agent.ToolCall
	seenIDs := make(map[string]struct{}, len(sdkMessage.ToolCalls))
	for i, sdkCall := range sdkMessage.ToolCalls {
		if sdkCall.Type != "function" {
			return agent.Message{}, fmt.Errorf(
				"deepseek: unsupported tool call type %q at index %d",
				sdkCall.Type,
				i,
			)
		}

		id := strings.TrimSpace(sdkCall.ID)
		if id == "" {
			return agent.Message{}, fmt.Errorf("deepseek: tool call ID cannot be empty at index %d", i)
		}
		if _, exists := seenIDs[id]; exists {
			return agent.Message{}, fmt.Errorf("deepseek: duplicate tool call ID in response: %q", id)
		}
		seenIDs[id] = struct{}{}

		name := strings.TrimSpace(sdkCall.Function.Name)
		if name == "" {
			return agent.Message{}, fmt.Errorf("deepseek: tool call function name cannot be empty at index %d", i)
		}

		arguments := []byte(sdkCall.Function.Arguments)
		if !json.Valid(arguments) {
			return agent.Message{}, fmt.Errorf(
				"deepseek: tool call arguments for %q must be valid JSON",
				name,
			)
		}

		toolCalls = append(toolCalls, agent.ToolCall{
			ID:        id,
			Name:      name,
			Arguments: append(json.RawMessage(nil), arguments...),
		})
	}

	if strings.TrimSpace(sdkMessage.Content) == "" && len(toolCalls) == 0 {
		return agent.Message{}, fmt.Errorf("deepseek: empty response content and no tool calls")
	}

	return agent.Message{
		Role:      agent.RoleAssistant,
		Content:   sdkMessage.Content,
		ToolCalls: toolCalls,
	}, nil
}
