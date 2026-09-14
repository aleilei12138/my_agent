package cmd

import (
	"context"
	"fmt"
	"my-project/internal/agent"
	"my-project/internal/config"
	"my-project/internal/llm/deepseek"
	"net/http"
)

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

		return agent.NewAgent(llmClient), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", llmConfig.Provider)
	}

}
