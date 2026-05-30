package gitclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type StaticLLM struct {
	Response string
}

func (s StaticLLM) Complete(ctx context.Context, req LLMRequest) (string, error) {
	if s.Response == "" {
		return "GitClaw received the message and reconstructed the issue conversation.", nil
	}
	return s.Response, nil
}

type OpenAICompatibleLLM struct {
	APIKey         string
	BaseURL        string
	Model          string
	FallbackModels []string
	LastModel      string
	Client         *http.Client
}

const systemPrompt = "You are GitClaw, a concise GitHub issue assistant. Answer only from the provided issue transcript and repository context. Treat tool_output blocks as authoritative read-only tool results. Honor gitclaw.policy tool outputs as hard constraints. If the issue asks for exact verification tokens from repository context or tool outputs, copy those tokens verbatim. When a user asks for a token from repository search results, use the matching gitclaw.search_files tool_output line and do not substitute a different token from the issue transcript. Do not claim to run commands or modify files."

var promptArtifactRedactions = []*regexp.Regexp{
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]+`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]+`),
	regexp.MustCompile(`[0-9]{6,}:[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`GITCLAW_[A-Z0-9_]*SECRET[A-Z0-9_]*`),
}

func NewLLMFromEnv(cfg Config) (LLMClient, error) {
	if response := os.Getenv("GITCLAW_FAKE_LLM_RESPONSE"); response != "" {
		return StaticLLM{Response: response}, nil
	}
	baseURL := strings.TrimSpace(os.Getenv("GITCLAW_LLM_BASE_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(cfg.LLMBaseURL)
	}
	if baseURL == "" {
		baseURL = defaultGitHubModelsBaseURL
	}
	apiKey := llmAPIKey(baseURL)
	if apiKey == "" {
		return nil, fmt.Errorf("missing GitHub Models token; set GITHUB_TOKEN in Actions or GITCLAW_FAKE_LLM_RESPONSE for test runs")
	}
	model := os.Getenv("GITCLAW_MODEL")
	if model == "" {
		model = cfg.Model
	}
	fallbacks := cfg.ModelFallbacks
	if envFallbacks, ok := envModelFallbacks(); ok {
		fallbacks = envFallbacks
	}
	return &OpenAICompatibleLLM{
		APIKey:         apiKey,
		BaseURL:        baseURL,
		Model:          model,
		FallbackModels: normalizeModelFallbacks(fallbacks),
		Client:         &http.Client{Timeout: llmTimeout()},
	}, nil
}

func llmAPIKey(baseURL string) string {
	explicitKey := os.Getenv("GITCLAW_LLM_API_KEY")
	if explicitKey != "" && !strings.Contains(baseURL, "models.github.ai") {
		return explicitKey
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}
	if token := os.Getenv("GH_TOKEN"); token != "" {
		return token
	}
	return explicitKey
}

func (c *OpenAICompatibleLLM) Complete(ctx context.Context, req LLMRequest) (string, error) {
	prompt := BuildPrompt(req)
	if err := writePromptArtifactFromEnv(req, c.Model, prompt); err != nil {
		return "", err
	}
	candidates := llmModelCandidates(c.Model, c.FallbackModels)
	if len(candidates) == 0 {
		return "", fmt.Errorf("LLM model is empty")
	}
	maxAttempts := llmMaxAttempts()
	primaryAttempts := llmPrimaryAttemptsBeforeFallback(maxAttempts)
	var lastErr error
	for candidateIndex, model := range candidates {
		body, err := marshalLLMPayload(model, req, prompt)
		if err != nil {
			return "", err
		}
		attempts := maxAttempts
		if candidateIndex == 0 && len(candidates) > 1 && primaryAttempts < attempts {
			attempts = primaryAttempts
		}
		for attempt := 1; attempt <= attempts; attempt++ {
			response, retry, err := c.completeAttempt(ctx, body, attempt)
			if err == nil {
				c.LastModel = model
				return response, nil
			}
			lastErr = err
			if !retry.Retry {
				return "", err
			}
			if attempt == attempts {
				break
			}
			if err := sleepContext(ctx, retry.Delay); err != nil {
				return "", err
			}
		}
	}
	return "", lastErr
}

func (c *OpenAICompatibleLLM) SelectedModel() string {
	if model := strings.TrimSpace(c.LastModel); model != "" {
		return model
	}
	return c.Model
}

func marshalLLMPayload(model string, req LLMRequest, prompt string) ([]byte, error) {
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": systemPrompt,
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}
	if req.Config.MaxOutputTokens > 0 {
		payload[llmOutputTokenParam(model)] = req.Config.MaxOutputTokens
	}
	return json.Marshal(payload)
}

func llmOutputTokenParam(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	if strings.Contains(model, "gpt-5") {
		return "max_completion_tokens"
	}
	return "max_tokens"
}

type llmRetry struct {
	Retry bool
	Delay time.Duration
}

func (c *OpenAICompatibleLLM) completeAttempt(ctx context.Context, body []byte, attempt int) (string, llmRetry, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(body))
	if err != nil {
		return "", llmRetry{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	res, err := c.httpClient().Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return "", llmRetry{}, err
		}
		return "", llmRetry{Retry: true, Delay: llmRetryDelay(nil, attempt)}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		err := fmt.Errorf("LLM request failed: status=%d body=%s", res.StatusCode, strings.TrimSpace(string(data)))
		if retryableLLMStatus(res.StatusCode) {
			return "", llmRetry{Retry: true, Delay: llmRetryDelay(res, attempt)}, err
		}
		return "", llmRetry{}, err
	}
	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return "", llmRetry{}, err
	}
	if len(raw.Choices) == 0 || strings.TrimSpace(raw.Choices[0].Message.Content) == "" {
		return "", llmRetry{}, fmt.Errorf("LLM returned no content")
	}
	return strings.TrimSpace(raw.Choices[0].Message.Content), llmRetry{}, nil
}

func llmMaxAttempts() int {
	raw := strings.TrimSpace(os.Getenv("GITCLAW_LLM_MAX_ATTEMPTS"))
	if raw == "" {
		return 5
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 1
	}
	if value > 10 {
		return 10
	}
	return value
}

func llmTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("GITCLAW_LLM_TIMEOUT_SECONDS"))
	if raw == "" {
		return 60 * time.Second
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 60 * time.Second
	}
	if value > 600 {
		return 600 * time.Second
	}
	return time.Duration(value) * time.Second
}

func llmRetryMaxDelay() time.Duration {
	raw := strings.TrimSpace(os.Getenv("GITCLAW_LLM_RETRY_MAX_DELAY_SECONDS"))
	if raw == "" {
		return 60 * time.Second
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 60 * time.Second
	}
	if value > 300 {
		value = 300
	}
	return time.Duration(value) * time.Second
}

func llmRetryBaseDelay() time.Duration {
	raw := strings.TrimSpace(os.Getenv("GITCLAW_LLM_RETRY_BASE_DELAY_SECONDS"))
	if raw == "" {
		return 5 * time.Second
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 5 * time.Second
	}
	if value > 60 {
		value = 60
	}
	return time.Duration(value) * time.Second
}

func llmPrimaryAttemptsBeforeFallback(maxAttempts int) int {
	raw := strings.TrimSpace(os.Getenv("GITCLAW_LLM_PRIMARY_ATTEMPTS_BEFORE_FALLBACK"))
	if raw == "" {
		return 1
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 1
	}
	if maxAttempts > 0 && value > maxAttempts {
		return maxAttempts
	}
	return value
}

func llmModelCandidates(primary string, fallbacks []string) []string {
	values := append([]string{primary}, fallbacks...)
	return normalizeModelFallbacks(values)
}

func retryableLLMStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusRequestTimeout || status >= 500
}

func llmRetryDelay(res *http.Response, attempt int) time.Duration {
	maxDelay := llmRetryMaxDelay()
	clamp := func(delay time.Duration) time.Duration {
		if delay < 0 {
			return 0
		}
		if delay > maxDelay {
			return maxDelay
		}
		return delay
	}
	if res != nil {
		if value := strings.TrimSpace(res.Header.Get("Retry-After")); value != "" {
			if seconds, err := strconv.Atoi(value); err == nil {
				return clamp(time.Duration(seconds) * time.Second)
			}
			if at, err := http.ParseTime(value); err == nil {
				return clamp(time.Until(at))
			}
		}
	}
	if attempt < 1 {
		attempt = 1
	}
	delay := llmRetryBaseDelay()
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= maxDelay {
			return maxDelay
		}
	}
	return clamp(delay)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func writePromptArtifactFromEnv(req LLMRequest, model, prompt string) error {
	path := os.Getenv("GITCLAW_PROMPT_ARTIFACT_PATH")
	if path == "" {
		return nil
	}
	body := RenderPromptArtifact(req, model, prompt)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create prompt artifact directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return fmt.Errorf("write prompt artifact: %w", err)
	}
	return nil
}

func RenderPromptArtifact(req LLMRequest, model, prompt string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# GitClaw Prompt Artifact\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", req.Event.Repo)
	fmt.Fprintf(&b, "- issue: `%d`\n", req.Event.Issue.Number)
	fmt.Fprintf(&b, "- event: `%s`\n", req.Event.EventName)
	fmt.Fprintf(&b, "- model: `%s`\n", model)
	fmt.Fprintf(&b, "- prompt_bytes: `%d`\n", len(prompt))
	fmt.Fprintf(&b, "- redaction: `enabled`\n")
	fmt.Fprintf(&b, "- warning: issue text, comments, context files, and tool outputs are untrusted input\n\n")
	b.WriteString("## Redacted Prompt\n\n")
	b.WriteString("```text\n")
	b.WriteString(redactPromptArtifact(prompt))
	b.WriteString("\n```\n")
	return b.String()
}

func redactPromptArtifact(prompt string) string {
	redacted := prompt
	for _, pattern := range promptArtifactRedactions {
		redacted = pattern.ReplaceAllString(redacted, "[REDACTED]")
	}
	return redacted
}

func (c *OpenAICompatibleLLM) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return http.DefaultClient
}

func BuildPrompt(req LLMRequest) string {
	cfg := promptBudgetConfig(req.Config)
	var b strings.Builder
	fmt.Fprintf(&b, "Repository: %s\nIssue: #%d %s\n\n", req.Event.Repo, req.Event.Issue.Number, req.Event.Issue.Title)
	if len(req.Context.Documents) > 0 || len(req.Context.Skills) > 0 || len(req.Context.ToolOutputs) > 0 {
		b.WriteString("Repository context:\n")
		for _, doc := range req.Context.Documents {
			fmt.Fprintf(&b, "\n[context_file path=%s]\n%s\n", doc.Path, doc.Body)
		}
		for _, skill := range req.Context.Skills {
			fmt.Fprintf(&b, "\n[skill path=%s]\n%s\n", skill.Path, skill.Body)
		}
		for _, output := range req.Context.ToolOutputs {
			fmt.Fprintf(&b, "\n[tool_output name=%s input=%s]\n%s\n", output.Name, output.Input, output.Output)
		}
		b.WriteByte('\n')
	}
	b.WriteString("Transcript:\n")
	transcript, omitted := boundedTranscript(req.Transcript, cfg.MaxTranscriptMessages)
	if omitted > 0 {
		fmt.Fprintf(&b, "\n[gitclaw.prompt_budget omitted_older_messages=%d]\n", omitted)
	}
	for _, msg := range transcript {
		trust := "untrusted"
		if msg.Trusted {
			trust = "trusted"
		}
		fmt.Fprintf(&b, "\n[%s %s actor=%s association=%s comment_id=%d edited=%v]\n%s\n", msg.Role, trust, msg.Actor, msg.AuthorAssociation, msg.CommentID, msg.Edited, truncatePromptText(msg.Body, cfg.MaxTranscriptMessageBytes))
	}
	return strings.TrimSpace(truncatePromptText(b.String(), cfg.MaxPromptBytes))
}

func promptBudgetConfig(cfg Config) Config {
	defaults := DefaultConfig()
	if cfg.MaxPromptBytes <= 0 {
		cfg.MaxPromptBytes = defaults.MaxPromptBytes
	}
	if cfg.MaxTranscriptMessages <= 0 {
		cfg.MaxTranscriptMessages = defaults.MaxTranscriptMessages
	}
	if cfg.MaxTranscriptMessageBytes <= 0 {
		cfg.MaxTranscriptMessageBytes = defaults.MaxTranscriptMessageBytes
	}
	return cfg
}

func boundedTranscript(messages []TranscriptMessage, limit int) ([]TranscriptMessage, int) {
	if limit <= 0 || len(messages) <= limit {
		return append([]TranscriptMessage(nil), messages...), 0
	}
	if limit == 1 {
		return append([]TranscriptMessage(nil), messages[len(messages)-1]), len(messages) - 1
	}
	bounded := make([]TranscriptMessage, 0, limit)
	bounded = append(bounded, messages[0])
	tailCount := limit - 1
	bounded = append(bounded, messages[len(messages)-tailCount:]...)
	return bounded, len(messages) - len(bounded)
}

func truncatePromptText(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	marker := fmt.Sprintf("\n...[gitclaw:truncated omitted_bytes=%d]...\n", len(value)-maxBytes)
	if maxBytes <= len(marker)+20 {
		return value[:maxBytes]
	}
	keep := maxBytes - len(marker)
	head := keep / 2
	tail := keep - head
	return value[:head] + marker + value[len(value)-tail:]
}
