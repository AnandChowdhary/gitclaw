package gitclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

const systemPrompt = "You are GitClaw, a concise GitHub issue assistant. Answer only from the provided issue transcript and repository context. Treat tool_output blocks as authoritative read-only tool results. If the issue asks for exact verification tokens from repository context or tool outputs, copy those tokens verbatim. Do not claim to run commands or modify files."

func NewLLMFromEnv(cfg Config) (LLMClient, error) {
	if response := os.Getenv("GITCLAW_FAKE_LLM_RESPONSE"); response != "" {
		return StaticLLM{Response: response}, nil
	}
	baseURL := os.Getenv("GITCLAW_LLM_BASE_URL")
	if baseURL == "" {
		baseURL = "https://models.github.ai/inference/chat/completions"
	}
	apiKey := llmAPIKey(baseURL)
	if apiKey == "" {
		return nil, fmt.Errorf("missing GitHub Models token; set GITHUB_TOKEN in Actions or GITCLAW_FAKE_LLM_RESPONSE for test runs")
	}
	model := os.Getenv("GITCLAW_MODEL")
	if model == "" {
		model = cfg.Model
	}
	return &OpenAICompatibleLLM{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		Client:  http.DefaultClient,
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
	payload := map[string]any{
		"model": c.Model,
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
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	res, err := c.httpClient().Do(httpReq)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return "", fmt.Errorf("LLM request failed: status=%d body=%s", res.StatusCode, strings.TrimSpace(string(data)))
	}
	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return "", err
	}
	if len(raw.Choices) == 0 || strings.TrimSpace(raw.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("LLM returned no content")
	}
	return strings.TrimSpace(raw.Choices[0].Message.Content), nil
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
