package gitclaw

import (
	"strings"
	"testing"
)

func TestDefaultConfigUsesGitHubModelsSmallDefault(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Model != "openai/gpt-5-mini" {
		t.Fatalf("default model = %q, want openai/gpt-5-mini", cfg.Model)
	}
}

func TestNewLLMFromEnvDefaultsToGitHubModelsWithActionsToken(t *testing.T) {
	t.Setenv("GITCLAW_FAKE_LLM_RESPONSE", "")
	t.Setenv("GITCLAW_LLM_API_KEY", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "github-token")
	t.Setenv("GITCLAW_LLM_BASE_URL", "")
	t.Setenv("GITCLAW_MODEL", "")

	llm, err := NewLLMFromEnv(DefaultConfig())
	if err != nil {
		t.Fatalf("NewLLMFromEnv returned error: %v", err)
	}
	client, ok := llm.(*OpenAICompatibleLLM)
	if !ok {
		t.Fatalf("llm type = %T, want *OpenAICompatibleLLM", llm)
	}
	if client.APIKey != "github-token" {
		t.Fatalf("APIKey = %q, want GitHub Actions token", client.APIKey)
	}
	if client.BaseURL != "https://models.github.ai/inference/chat/completions" {
		t.Fatalf("BaseURL = %q, want GitHub Models endpoint", client.BaseURL)
	}
	if client.Model != "openai/gpt-5-mini" {
		t.Fatalf("Model = %q, want openai/gpt-5-mini", client.Model)
	}
}

func TestNewLLMFromEnvSupportsOpenAICompatibleOverride(t *testing.T) {
	t.Setenv("GITCLAW_FAKE_LLM_RESPONSE", "")
	t.Setenv("GITHUB_TOKEN", "github-token")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITCLAW_LLM_API_KEY", "external-token")
	t.Setenv("GITCLAW_LLM_BASE_URL", "https://llm.example.test/chat/completions")
	t.Setenv("GITCLAW_MODEL", "example/model")

	llm, err := NewLLMFromEnv(DefaultConfig())
	if err != nil {
		t.Fatalf("NewLLMFromEnv returned error: %v", err)
	}
	client := llm.(*OpenAICompatibleLLM)
	if client.APIKey != "external-token" {
		t.Fatalf("APIKey = %q, want explicit external token", client.APIKey)
	}
	if client.BaseURL != "https://llm.example.test/chat/completions" {
		t.Fatalf("BaseURL = %q, want explicit base URL", client.BaseURL)
	}
	if client.Model != "example/model" {
		t.Fatalf("Model = %q, want explicit model", client.Model)
	}
}

func TestSystemPromptNamesToolOutputsAndExactTokens(t *testing.T) {
	for _, want := range []string{"tool_output", "exact verification tokens", "copy those tokens verbatim"} {
		if !strings.Contains(systemPrompt, want) {
			t.Fatalf("system prompt missing %q: %s", want, systemPrompt)
		}
	}
}
