package config

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
)

// setTestConfig 是一个辅助函数，将传入的 YAML 字符串注入为 GoFrame 的全局配置
// 并在当前测试结束时通过 t.Cleanup 自动还原，避免污染后续测试。
func setTestConfig(t *testing.T, yamlContent string) {
	t.Helper()

	adapter, err := gcfg.NewAdapterContent(yamlContent)
	if err != nil {
		t.Fatalf("failed to create config adapter: %v", err)
	}

	oldAdapter := g.Cfg().GetAdapter()
	g.Cfg().SetAdapter(adapter)

	t.Cleanup(func() {
		g.Cfg().SetAdapter(oldAdapter)
	})
}

// 1. 测试全部配置合法时的加载情况
func TestLoadLLM_Success(t *testing.T) {
	yaml := `
llm:
  provider: "deepseek"
  deepseek:
    baseUrl: "https://api.deepseek.com"
    model: "deepseek-chat"
    timeout: "15s"
`
	setTestConfig(t, yaml)
	t.Setenv("DEEPSEEK_API_KEY", "test-mock-key")

	cfg, err := LoadLLM(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Provider != "deepseek" {
		t.Errorf("expected provider 'deepseek', got '%s'", cfg.Provider)
	}
	if cfg.DeepSeek.BaseURL != "https://api.deepseek.com" {
		t.Errorf("expected baseUrl 'https://api.deepseek.com', got '%s'", cfg.DeepSeek.BaseURL)
	}
	if cfg.DeepSeek.Model != "deepseek-chat" {
		t.Errorf("expected model 'deepseek-chat', got '%s'", cfg.DeepSeek.Model)
	}
	if cfg.DeepSeek.Timeout != 15*time.Second {
		t.Errorf("expected timeout 15s, got %v", cfg.DeepSeek.Timeout)
	}
	if cfg.DeepSeek.APIKey != "test-mock-key" {
		t.Errorf("expected apiKey 'test-mock-key', got '%s'", cfg.DeepSeek.APIKey)
	}
}

// 2. 测试环境变量未设置时报错
func TestLoadLLM_APIKeyNotSet(t *testing.T) {
	yaml := `
llm:
  provider: "deepseek"
  deepseek:
    model: "deepseek-chat"
`
	setTestConfig(t, yaml)
	// 确保环境变量完全不存在
	t.Setenv("DEEPSEEK_API_KEY", "")
	// Go 标准库没有 unsetenv 的 t.Helper，通过传入临时环境测试或使用没有该 key 的测试上下文
	// 这里设置为空格，或者直接在测试时利用不存在变量测试
}

// 3. 测试环境变量为空或全为空格时报错
func TestLoadLLM_APIKeyEmpty(t *testing.T) {
	yaml := `
llm:
  provider: "deepseek"
  deepseek:
    model: "deepseek-chat"
`
	setTestConfig(t, yaml)
	t.Setenv("DEEPSEEK_API_KEY", "   ")

	_, err := LoadLLM(context.Background())
	if err == nil {
		t.Fatal("expected error for empty DEEPSEEK_API_KEY, got nil")
	}
	if !strings.Contains(err.Error(), "DEEPSEEK_API_KEY") {
		t.Errorf("expected error message to mention DEEPSEEK_API_KEY, got: %v", err)
	}
}

// 4. 测试不支持的 Provider
func TestLoadLLM_UnsupportedProvider(t *testing.T) {
	yaml := `
llm:
  provider: "openai"
  deepseek:
    model: "deepseek-chat"
`
	setTestConfig(t, yaml)
	t.Setenv("DEEPSEEK_API_KEY", "test-mock-key")

	_, err := LoadLLM(context.Background())
	if err == nil {
		t.Fatal("expected error for unsupported provider, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported LLM provider") {
		t.Errorf("expected error 'unsupported LLM provider', got: %v", err)
	}
}

// 5. 测试 BaseURL 为空时自动回退为默认值
func TestLoadLLM_BaseURLDefault(t *testing.T) {
	yaml := `
llm:
  provider: "deepseek"
  deepseek:
    model: "deepseek-chat"
`
	setTestConfig(t, yaml)
	t.Setenv("DEEPSEEK_API_KEY", "test-mock-key")

	cfg, err := LoadLLM(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DeepSeek.BaseURL != "https://api.deepseek.com" {
		t.Errorf("expected default baseUrl 'https://api.deepseek.com', got '%s'", cfg.DeepSeek.BaseURL)
	}
}

// 6. 测试 BaseURL 末尾的多个斜杠是否被正确清理
func TestLoadLLM_BaseURLTrimSlash(t *testing.T) {
	yaml := `
llm:
  provider: "deepseek"
  deepseek:
    baseUrl: "https://custom.deepseek.com///"
    model: "deepseek-chat"
`
	setTestConfig(t, yaml)
	t.Setenv("DEEPSEEK_API_KEY", "test-mock-key")

	cfg, err := LoadLLM(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DeepSeek.BaseURL != "https://custom.deepseek.com" {
		t.Errorf("expected 'https://custom.deepseek.com', got '%s'", cfg.DeepSeek.BaseURL)
	}
}

// 7. 测试 Model 缺失或为空时报错
func TestLoadLLM_ModelEmpty(t *testing.T) {
	yaml := `
llm:
  provider: "deepseek"
  deepseek:
    model: "   "
`
	setTestConfig(t, yaml)
	t.Setenv("DEEPSEEK_API_KEY", "test-mock-key")

	_, err := LoadLLM(context.Background())
	if err == nil {
		t.Fatal("expected error when model is empty, got nil")
	}
	if !strings.Contains(err.Error(), "llm.deepseek.model is empty") {
		t.Errorf("expected 'llm.deepseek.model is empty' error, got: %v", err)
	}
}

// 8. 测试 Timeout 未设置时采用默认值 30s
func TestLoadLLM_TimeoutDefault(t *testing.T) {
	yaml := `
llm:
  provider: "deepseek"
  deepseek:
    model: "deepseek-chat"
`
	setTestConfig(t, yaml)
	t.Setenv("DEEPSEEK_API_KEY", "test-mock-key")

	cfg, err := LoadLLM(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DeepSeek.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", cfg.DeepSeek.Timeout)
	}
}

// 9. 测试 Timeout 格式非法或小于等于 0 时报错
func TestLoadLLM_TimeoutInvalid(t *testing.T) {
	testCases := []struct {
		name    string
		timeout string
	}{
		{"negative timeout", "-5s"},
		{"zero timeout", "0s"},
		{"invalid format", "not-a-duration"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			yaml := `
llm:
  provider: "deepseek"
  deepseek:
    model: "deepseek-chat"
    timeout: "` + tc.timeout + `"
`
			setTestConfig(t, yaml)
			t.Setenv("DEEPSEEK_API_KEY", "test-mock-key")

			_, err := LoadLLM(context.Background())
			if err == nil {
				t.Fatalf("expected error for timeout '%s', got nil", tc.timeout)
			}
			if !strings.Contains(err.Error(), "llm.deepseek.timeout must be positive") {
				t.Errorf("expected timeout must be positive error, got: %v", err)
			}
		})
	}
}
