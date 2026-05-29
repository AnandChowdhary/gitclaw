package gitclaw

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

const defaultGitHubModelsBaseURL = "https://models.github.ai/inference/chat/completions"

func IsModelReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/model" || command == "/models"
}

func RenderModelReport(ev Event, cfg Config) string {
	baseURL := llmBaseURL(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw Model Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- provider: `%s`\n", llmProviderForReport(cfg, baseURL))
	fmt.Fprintf(&b, "- model: `%s`\n", cfg.Model)
	fmt.Fprintf(&b, "- endpoint_host: `%s`\n", llmEndpointHost(baseURL))
	fmt.Fprintf(&b, "- token_source: `%s`\n", llmTokenSource(baseURL))
	fmt.Fprintf(&b, "- request_timeout_seconds: `%d`\n", int(llmTimeout().Seconds()))
	fmt.Fprintf(&b, "- retry_max_attempts: `%d`\n", llmMaxAttempts())
	fmt.Fprintf(&b, "- retry_base_delay_seconds: `%d`\n", int(llmRetryBaseDelay().Seconds()))
	fmt.Fprintf(&b, "- retry_max_delay_seconds: `%d`\n", int(llmRetryMaxDelay().Seconds()))
	fmt.Fprintf(&b, "- retryable_statuses: `%s`\n", "429, 408, 5xx")
	fmt.Fprintf(&b, "- prompt_artifact_enabled: `%t`\n", strings.TrimSpace(os.Getenv("GITCLAW_PROMPT_ARTIFACT_PATH")) != "")
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n\n", shortDocumentHash(ev.Issue.Title))

	b.WriteString("The model client retries transient provider failures with bounded exponential backoff and honors bounded `Retry-After` values when providers return them.\n\n")
	b.WriteString("Issue bodies, comment bodies, API keys, and raw provider error bodies are not included in this report.\n\n")
	b.WriteString("### Environment Knobs\n")
	b.WriteString("- `GITCLAW_MODEL`\n")
	b.WriteString("- `GITCLAW_LLM_BASE_URL`\n")
	b.WriteString("- `GITCLAW_LLM_TIMEOUT_SECONDS`\n")
	b.WriteString("- `GITCLAW_LLM_MAX_ATTEMPTS`\n")
	b.WriteString("- `GITCLAW_LLM_RETRY_BASE_DELAY_SECONDS`\n")
	b.WriteString("- `GITCLAW_LLM_RETRY_MAX_DELAY_SECONDS`\n")

	return strings.TrimSpace(b.String())
}

func llmProviderForReport(cfg Config, baseURL string) string {
	if os.Getenv("GITCLAW_LLM_BASE_URL") != "" || strings.TrimSpace(cfg.ModelProvider) == "" {
		return llmProviderName(baseURL)
	}
	return cfg.ModelProvider
}

func llmBaseURL(cfg Config) string {
	baseURL := strings.TrimSpace(os.Getenv("GITCLAW_LLM_BASE_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(cfg.LLMBaseURL)
	}
	if baseURL == "" {
		baseURL = defaultGitHubModelsBaseURL
	}
	return baseURL
}

func llmProviderName(baseURL string) string {
	if strings.Contains(baseURL, "models.github.ai") {
		return "github-models"
	}
	return "openai-compatible"
}

func llmEndpointHost(baseURL string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" {
		return "custom"
	}
	return parsed.Host
}

func llmTokenSource(baseURL string) string {
	if strings.TrimSpace(os.Getenv("GITHUB_TOKEN")) != "" {
		return "GITHUB_TOKEN"
	}
	if strings.TrimSpace(os.Getenv("GH_TOKEN")) != "" {
		return "GH_TOKEN"
	}
	if strings.TrimSpace(os.Getenv("GITCLAW_LLM_API_KEY")) != "" {
		return "GITCLAW_LLM_API_KEY"
	}
	return "none"
}
