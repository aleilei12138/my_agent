package cmd

import (
	"context"
	"fmt"
	"my-project/internal/agent"
	"my-project/internal/config"
	"my-project/internal/llm/deepseek"
	"net/http"
)

const defaultMaxTurns = 8

func newAgentFromConfig(ctx context.Context) (*agent.Agent, error) {
	llmConfig, err := config.LoadLLM(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load LLM config: %w", err)
	}

	httpClient := &http.Client{
		Timeout: llmConfig.DeepSeek.Timeout,
	}

	switch llmConfig.Provider {
	case "deepseek":
		llmClient, err := deepseek.NewClient(deepseek.Config{
			BaseURL:    llmConfig.DeepSeek.BaseURL,
			APIKey:     llmConfig.DeepSeek.APIKey,
			Model:      llmConfig.DeepSeek.Model,
			HTTPClient: httpClient,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create DeepSeek client: %w", err)
		}
		registry, err := agent.NewToolRegistry()
		if err != nil {
			return nil, fmt.Errorf("failed to create tool registry: %w", err)
		}
		agentInstance, err := agent.NewAgent(llmClient, registry, agent.Config{MaxTurns: defaultMaxTurns})
		if err != nil {
			return nil, fmt.Errorf("failed to create agent: %w", err)
		}
		return agentInstance, nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", llmConfig.Provider)
	}

}
