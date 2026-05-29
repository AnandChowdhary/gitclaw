package gitclaw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestSystemPromptNamesToolOutputsAndExactTokens(t *testing.T) {
	for _, want := range []string{"tool_output", "gitclaw.policy", "hard constraints", "exact verification tokens", "copy those tokens verbatim"} {
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
	}, "openai/gpt-5-mini", prompt)
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
