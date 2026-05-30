package gitclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfigUsesGitHubModelsSmallDefault(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Model != "openai/gpt-5-nano" {
		t.Fatalf("default model = %q, want openai/gpt-5-nano", cfg.Model)
	}
}

func TestNewLLMFromEnvDefaultsToGitHubModelsWithActionsToken(t *testing.T) {
	t.Setenv("GITCLAW_FAKE_LLM_RESPONSE", "")
	t.Setenv("GITCLAW_LLM_API_KEY", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "github-token")
	t.Setenv("GITCLAW_LLM_BASE_URL", "")
	t.Setenv("GITCLAW_MODEL", "")
	t.Setenv("GITCLAW_LLM_MAX_ATTEMPTS", "")
	t.Setenv("GITCLAW_LLM_RETRY_MAX_DELAY_SECONDS", "")
	t.Setenv("GITCLAW_LLM_TIMEOUT_SECONDS", "")

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
	if client.Model != "openai/gpt-5-nano" {
		t.Fatalf("Model = %q, want openai/gpt-5-nano", client.Model)
	}
	if len(client.FallbackModels) != 0 {
		t.Fatalf("FallbackModels = %#v, want none", client.FallbackModels)
	}
	if client.Client.Timeout != time.Minute {
		t.Fatalf("client timeout = %s, want 1m0s", client.Client.Timeout)
	}
	if llmMaxAttempts() != 5 {
		t.Fatalf("llmMaxAttempts() = %d, want 5", llmMaxAttempts())
	}
	if llmRetryMaxDelay() != time.Minute {
		t.Fatalf("llmRetryMaxDelay() = %s, want 1m0s", llmRetryMaxDelay())
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

func TestNewLLMFromEnvIncludesConfiguredFallbacks(t *testing.T) {
	t.Setenv("GITCLAW_FAKE_LLM_RESPONSE", "")
	t.Setenv("GITHUB_TOKEN", "github-token")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITCLAW_LLM_API_KEY", "")
	t.Setenv("GITCLAW_LLM_BASE_URL", "")
	t.Setenv("GITCLAW_MODEL", "")
	cfg := DefaultConfig()
	cfg.ModelFallbacks = []string{"openai/gpt-4.1-nano", "openai/gpt-4.1-nano"}

	llm, err := NewLLMFromEnv(cfg)
	if err != nil {
		t.Fatalf("NewLLMFromEnv returned error: %v", err)
	}
	client := llm.(*OpenAICompatibleLLM)
	if len(client.FallbackModels) != 1 || client.FallbackModels[0] != "openai/gpt-4.1-nano" {
		t.Fatalf("FallbackModels = %#v, want normalized configured fallback", client.FallbackModels)
	}
}

func TestNewLLMFromEnvAllowsFallbackEnvOverride(t *testing.T) {
	t.Setenv("GITCLAW_FAKE_LLM_RESPONSE", "")
	t.Setenv("GITHUB_TOKEN", "github-token")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITCLAW_LLM_API_KEY", "")
	t.Setenv("GITCLAW_LLM_BASE_URL", "")
	t.Setenv("GITCLAW_MODEL", "")
	t.Setenv("GITCLAW_MODEL_FALLBACKS", "none")
	cfg := DefaultConfig()
	cfg.ModelFallbacks = []string{"openai/gpt-4.1-nano"}

	llm, err := NewLLMFromEnv(cfg)
	if err != nil {
		t.Fatalf("NewLLMFromEnv returned error: %v", err)
	}
	client := llm.(*OpenAICompatibleLLM)
	if len(client.FallbackModels) != 0 {
		t.Fatalf("FallbackModels = %#v, want env disabled fallback", client.FallbackModels)
	}
}

func TestOpenAICompatibleLLMRetriesRateLimit(t *testing.T) {
	t.Setenv("GITCLAW_LLM_MAX_ATTEMPTS", "2")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"retried ok"}}]}`))
	}))
	defer server.Close()

	llm := &OpenAICompatibleLLM{
		APIKey:  "token",
		BaseURL: server.URL,
		Model:   "test-model",
		Client:  server.Client(),
	}
	got, err := llm.Complete(context.Background(), LLMRequest{
		Event: Event{Repo: "owner/repo", Issue: Issue{Number: 1, Title: "retry"}},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if got != "retried ok" {
		t.Fatalf("Complete = %q, want retried ok", got)
	}
	if calls != 2 {
		t.Fatalf("server calls = %d, want 2", calls)
	}
}

func TestOpenAICompatibleLLMFallsBackAfterRetryablePrimaryFailure(t *testing.T) {
	t.Setenv("GITCLAW_LLM_MAX_ATTEMPTS", "3")
	t.Setenv("GITCLAW_LLM_PRIMARY_ATTEMPTS_BEFORE_FALLBACK", "1")
	var models []string
	var fallbackPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		model, _ := payload["model"].(string)
		models = append(models, model)
		if model == "openai/gpt-5-nano" {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		fallbackPayload = payload
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"fallback ok"}}]}`))
	}))
	defer server.Close()

	llm := &OpenAICompatibleLLM{
		APIKey:         "token",
		BaseURL:        server.URL,
		Model:          "openai/gpt-5-nano",
		FallbackModels: []string{"openai/gpt-4.1-nano"},
		Client:         server.Client(),
	}
	got, err := llm.Complete(context.Background(), LLMRequest{
		Event:  Event{Repo: "owner/repo", Issue: Issue{Number: 1, Title: "fallback"}},
		Config: Config{MaxOutputTokens: 123},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if got != "fallback ok" {
		t.Fatalf("Complete = %q, want fallback ok", got)
	}
	if strings.Join(models, ",") != "openai/gpt-5-nano,openai/gpt-4.1-nano" {
		t.Fatalf("models called = %#v, want primary then fallback", models)
	}
	if llm.SelectedModel() != "openai/gpt-4.1-nano" {
		t.Fatalf("SelectedModel = %q, want fallback", llm.SelectedModel())
	}
	if got := fallbackPayload["max_tokens"]; got != float64(123) {
		t.Fatalf("fallback max_tokens = %#v, want 123", got)
	}
	if _, ok := fallbackPayload["max_completion_tokens"]; ok {
		t.Fatalf("fallback payload unexpectedly included max_completion_tokens: %#v", fallbackPayload)
	}
}

func TestOpenAICompatibleLLMDoesNotFallbackForNonRetryablePrimaryFailure(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "bad model", http.StatusBadRequest)
	}))
	defer server.Close()

	llm := &OpenAICompatibleLLM{
		APIKey:         "token",
		BaseURL:        server.URL,
		Model:          "openai/not-real",
		FallbackModels: []string{"openai/gpt-4.1-nano"},
		Client:         server.Client(),
	}
	_, err := llm.Complete(context.Background(), LLMRequest{
		Event: Event{Repo: "owner/repo", Issue: Issue{Number: 1, Title: "no fallback"}},
	})
	if err == nil {
		t.Fatalf("Complete should return non-retryable error")
	}
	if calls != 1 {
		t.Fatalf("server calls = %d, want one primary call only", calls)
	}
}

func TestOpenAICompatibleLLMSendsMaxOutputTokensFromConfig(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	llm := &OpenAICompatibleLLM{
		APIKey:  "token",
		BaseURL: server.URL,
		Model:   "test-model",
		Client:  server.Client(),
	}
	_, err := llm.Complete(context.Background(), LLMRequest{
		Event:  Event{Repo: "owner/repo", Issue: Issue{Number: 1, Title: "max tokens"}},
		Config: Config{MaxOutputTokens: 321},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if got := payload["max_tokens"]; got != float64(321) {
		t.Fatalf("max_tokens = %#v, want 321", got)
	}
}

func TestOpenAICompatibleLLMUsesCompletionTokenParameterForGPT5(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	llm := &OpenAICompatibleLLM{
		APIKey:  "token",
		BaseURL: server.URL,
		Model:   "openai/gpt-5-nano",
		Client:  server.Client(),
	}
	_, err := llm.Complete(context.Background(), LLMRequest{
		Event:  Event{Repo: "owner/repo", Issue: Issue{Number: 1, Title: "max completion tokens"}},
		Config: Config{MaxOutputTokens: 321},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if got := payload["max_completion_tokens"]; got != float64(321) {
		t.Fatalf("max_completion_tokens = %#v, want 321", got)
	}
	if _, ok := payload["max_tokens"]; ok {
		t.Fatalf("payload unexpectedly included max_tokens: %#v", payload)
	}
}

func TestLLMTimeoutFromEnvIsBounded(t *testing.T) {
	t.Setenv("GITCLAW_LLM_TIMEOUT_SECONDS", "999")
	if got := llmTimeout(); got != 10*time.Minute {
		t.Fatalf("llmTimeout() = %s, want 10m0s", got)
	}
	t.Setenv("GITCLAW_LLM_TIMEOUT_SECONDS", "7")
	if got := llmTimeout(); got != 7*time.Second {
		t.Fatalf("llmTimeout() = %s, want 7s", got)
	}
}

func TestLLMRetryDelayCapsRetryAfter(t *testing.T) {
	t.Setenv("GITCLAW_LLM_RETRY_MAX_DELAY_SECONDS", "2")
	res := &http.Response{Header: http.Header{"Retry-After": []string{"120"}}}
	if got := llmRetryDelay(res, 1); got != 2*time.Second {
		t.Fatalf("llmRetryDelay() = %s, want 2s", got)
	}
}

func TestLLMRetryDelayUsesBoundedExponentialBackoff(t *testing.T) {
	t.Setenv("GITCLAW_LLM_RETRY_BASE_DELAY_SECONDS", "3")
	t.Setenv("GITCLAW_LLM_RETRY_MAX_DELAY_SECONDS", "10")
	for _, tc := range []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: 3 * time.Second},
		{attempt: 2, want: 6 * time.Second},
		{attempt: 3, want: 10 * time.Second},
	} {
		if got := llmRetryDelay(nil, tc.attempt); got != tc.want {
			t.Fatalf("attempt %d delay = %s, want %s", tc.attempt, got, tc.want)
		}
	}
}

func TestLLMPrimaryAttemptsBeforeFallbackIsBounded(t *testing.T) {
	t.Setenv("GITCLAW_LLM_PRIMARY_ATTEMPTS_BEFORE_FALLBACK", "")
	if got := llmPrimaryAttemptsBeforeFallback(6); got != 1 {
		t.Fatalf("default primary attempts = %d, want 1", got)
	}
	t.Setenv("GITCLAW_LLM_PRIMARY_ATTEMPTS_BEFORE_FALLBACK", "99")
	if got := llmPrimaryAttemptsBeforeFallback(6); got != 6 {
		t.Fatalf("bounded primary attempts = %d, want 6", got)
	}
	t.Setenv("GITCLAW_LLM_PRIMARY_ATTEMPTS_BEFORE_FALLBACK", "0")
	if got := llmPrimaryAttemptsBeforeFallback(6); got != 1 {
		t.Fatalf("invalid primary attempts = %d, want 1", got)
	}
}

func TestSystemPromptNamesToolOutputsAndExactTokens(t *testing.T) {
	for _, want := range []string{"tool_output", "gitclaw.policy", "hard constraints", "exact verification tokens", "copy those tokens verbatim", "gitclaw.search_files", "do not substitute"} {
		if !strings.Contains(systemPrompt, want) {
			t.Fatalf("system prompt missing %q: %s", want, systemPrompt)
		}
	}
}

func TestBuildPromptBoundsTranscriptMessagesAndBodies(t *testing.T) {
	messages := []TranscriptMessage{{
		Role: "user",
		Body: "original issue should remain",
	}}
	for i := 1; i <= 5; i++ {
		body := strings.Repeat("noise ", 60)
		if i == 5 {
			body += "TAIL_TOKEN_SHOULD_SURVIVE"
		}
		messages = append(messages, TranscriptMessage{
			Role:      "user",
			Body:      body,
			CommentID: int64(i),
		})
	}
	prompt := BuildPrompt(LLMRequest{
		Event: Event{Repo: "owner/repo", Issue: Issue{Number: 1, Title: "@gitclaw budget"}},
		Config: Config{
			MaxPromptBytes:            4000,
			MaxTranscriptMessages:     3,
			MaxTranscriptMessageBytes: 140,
		},
		Transcript: messages,
	})
	for _, want := range []string{"original issue should remain", "omitted_older_messages=3", "gitclaw:truncated", "TAIL_TOKEN_SHOULD_SURVIVE"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "comment_id=2") {
		t.Fatalf("older middle transcript message was not omitted:\n%s", prompt)
	}
}

func TestBuildPromptEnforcesMaxPromptBytes(t *testing.T) {
	prompt := BuildPrompt(LLMRequest{
		Event: Event{Repo: "owner/repo", Issue: Issue{Number: 2, Title: "@gitclaw budget"}},
		Config: Config{
			MaxPromptBytes:            700,
			MaxTranscriptMessages:     5,
			MaxTranscriptMessageBytes: 1000,
		},
		Context: RepoContext{
			Documents: []ContextDocument{{Path: "huge.md", Body: strings.Repeat("context ", 300)}},
		},
		Transcript: []TranscriptMessage{{
			Role: "user",
			Body: strings.Repeat("body ", 100) + "FINAL_TAIL_TOKEN",
		}},
	})
	if len(prompt) > 700 {
		t.Fatalf("prompt len = %d, want <= 700", len(prompt))
	}
	for _, want := range []string{"gitclaw:truncated", "FINAL_TAIL_TOKEN"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("bounded prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestRenderPromptArtifactRedactsSecrets(t *testing.T) {
	secret := "GITCLAW_ARTIFACT_SECRET_20260529"
	prompt := "User asked with " + secret + " and token ghp_abcdefghijklmnopqrstuvwxyz123456"
	artifact := RenderPromptArtifact(LLMRequest{
		Event: Event{
			Repo:      "owner/repo",
			EventName: "issues",
			Issue:     Issue{Number: 12},
		},
	}, "openai/gpt-5-nano", prompt)
	for _, notWant := range []string{secret, "ghp_abcdefghijklmnopqrstuvwxyz123456"} {
		if strings.Contains(artifact, notWant) {
			t.Fatalf("artifact leaked %q:\n%s", notWant, artifact)
		}
	}
	for _, want := range []string{"redaction: `enabled`", "untrusted input", "[REDACTED]"} {
		if !strings.Contains(artifact, want) {
			t.Fatalf("artifact missing %q:\n%s", want, artifact)
		}
	}
}

func TestWritePromptArtifactFromEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prompt.md")
	t.Setenv("GITCLAW_PROMPT_ARTIFACT_PATH", path)
	err := writePromptArtifactFromEnv(LLMRequest{
		Event: Event{Repo: "owner/repo", EventName: "issues", Issue: Issue{Number: 2}},
	}, "model", "prompt")
	if err != nil {
		t.Fatalf("writePromptArtifactFromEnv returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if !strings.Contains(string(data), "GitClaw Prompt Artifact") {
		t.Fatalf("unexpected artifact body:\n%s", data)
	}
}
