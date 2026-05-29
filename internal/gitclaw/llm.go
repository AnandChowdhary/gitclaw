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
				"content": "You are GitClaw, a concise GitHub issue assistant. Answer only from the provided issue transcript and repository context. Do not claim to run commands or modify files.",
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
	var b strings.Builder
	fmt.Fprintf(&b, "Repository: %s\nIssue: #%d %s\n\n", req.Event.Repo, req.Event.Issue.Number, req.Event.Issue.Title)
	b.WriteString("Transcript:\n")
	for _, msg := range req.Transcript {
		trust := "untrusted"
		if msg.Trusted {
			trust = "trusted"
		}
		fmt.Fprintf(&b, "\n[%s %s actor=%s association=%s comment_id=%d edited=%v]\n%s\n", msg.Role, trust, msg.Actor, msg.AuthorAssociation, msg.CommentID, msg.Edited, msg.Body)
	}
	return strings.TrimSpace(b.String())
}
