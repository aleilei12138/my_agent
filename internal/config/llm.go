package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	defaultBaseURL = "https://api.deepseek.com"
	defaultTimeout = 30 * time.Second
)

// LLMConfig 代表大模型模块的总体运行时配置
type LLMConfig struct {
	Provider string
	DeepSeek DeepSeekConfig
}

// DeepSeekConfig 代表 DeepSeek 适配器运行所需的具体参数
type DeepSeekConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

// LoadLLM 从配置系统和环境变量中加载并校验 LLM 配置
func LoadLLM(ctx context.Context) (LLMConfig, error) {
	// 1. 读取并校验 API Key (仅限环境变量)
	apiKey, ok := os.LookupEnv("DEEPSEEK_API_KEY")
	if !ok {
		return LLMConfig{}, errors.New("DEEPSEEK_API_KEY is not set")
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return LLMConfig{}, errors.New("DEEPSEEK_API_KEY is empty")
	}

	// 2. 读取并校验 Provider
	v, err := g.Cfg().Get(ctx, "llm.provider")
	if err != nil {
		return LLMConfig{}, fmt.Errorf("load llm.provider: %w", err)
	}
	if v.IsEmpty() {
		return LLMConfig{}, errors.New("llm.provider is not set in config")
	}
	provider := strings.TrimSpace(v.String())
	if provider == "" {
		return LLMConfig{}, errors.New("llm.provider is empty")
	}
	if provider != "deepseek" {
		return LLMConfig{}, fmt.Errorf("unsupported LLM provider: %s", provider)
	}

	// 3. 读取 BaseURL (支持默认值并去除末尾斜杠)
	v, err = g.Cfg().Get(ctx, "llm.deepseek.baseUrl")
	if err != nil {
		return LLMConfig{}, fmt.Errorf("load llm.deepseek.baseUrl: %w", err)
	}
	baseURL := defaultBaseURL
	if !v.IsEmpty() && strings.TrimSpace(v.String()) != "" {
		baseURL = strings.TrimSpace(v.String())
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return LLMConfig{}, errors.New("llm.deepseek.baseUrl is empty")
	}

	// 4. 读取 Model (必填项)
	v, err = g.Cfg().Get(ctx, "llm.deepseek.model")
	if err != nil {
		return LLMConfig{}, fmt.Errorf("load llm.deepseek.model: %w", err)
	}
	if v.IsEmpty() {
		return LLMConfig{}, errors.New("llm.deepseek.model is not set in config")
	}
	model := strings.TrimSpace(v.String())
	if model == "" {
		return LLMConfig{}, errors.New("llm.deepseek.model is empty")
	}

	// 5. 读取 Timeout (支持默认值，必须大于 0)
	v, err = g.Cfg().Get(ctx, "llm.deepseek.timeout")
	if err != nil {
		return LLMConfig{}, fmt.Errorf("load llm.deepseek.timeout: %w", err)
	}
	timeout := defaultTimeout
	if !v.IsEmpty() && strings.TrimSpace(v.String()) != "" {
		timeout = v.Duration()
	}
	if timeout <= 0 {
		return LLMConfig{}, errors.New("llm.deepseek.timeout must be positive")
	}

	return LLMConfig{
		Provider: provider,
		DeepSeek: DeepSeekConfig{
			BaseURL: baseURL,
			APIKey:  apiKey,
			Model:   model,
			Timeout: timeout,
		},
	}, nil
}
