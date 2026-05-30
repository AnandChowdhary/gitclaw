package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestModelRiskCommandReportsCurrentSafeSurfaceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeSafeModelRiskConfig(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)
	t.Setenv("GITHUB_TOKEN", "MODEL_RISK_SAFE_TOKEN")
	t.Setenv("GITCLAW_LLM_MAX_ATTEMPTS", "6")
	t.Setenv("GITCLAW_LLM_TIMEOUT_SECONDS", "75")
	t.Setenv("GITCLAW_LLM_RETRY_BASE_DELAY_SECONDS", "10")
	t.Setenv("GITCLAW_LLM_RETRY_MAX_DELAY_SECONDS", "90")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"models", "risk"}); err != nil {
			t.Fatalf("models risk returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Model Risk Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"model_risk_status: `ok`",
		"verification_scope: `github_models_control_plane`",
		"provider: `github-models`",
		"model: `openai/gpt-5-nano`",
		"fallback_models: `openai/gpt-4.1-nano`",
		"fallback_models_configured: `1`",
		"default_model_policy: `smallest-openai-github-models-catalog-model`",
		"catalog_endpoint_host: `models.github.ai`",
		"endpoint_host: `models.github.ai`",
		"token_source: `GITHUB_TOKEN`",
		"output_token_parameter: `max_completion_tokens`",
		"request_timeout_seconds: `75`",
		"retry_max_attempts: `6`",
		"retry_base_delay_seconds: `10`",
		"retry_max_delay_seconds: `90`",
		"retryable_statuses: `429, 408, 5xx`",
		"fallback_on_retryable_statuses: `true`",
		"fallback_primary_attempts_before_fallback: `1`",
		"prompt_artifact_enabled: `false`",
		"config_file_present: `true`",
		"config_file_path: `.gitclaw/config.yml`",
		"github_models_endpoint: `true`",
		"openai_compatible_endpoint: `true`",
		"github_actions_token_supported: `true`",
		"primary_model_small_default: `true`",
		"primary_model_known_github_catalog_entry: `true`",
		"fallback_models_known_github_catalog_entries: `1`",
		"model_catalog_probe_performed: `false`",
		"live_inference_probe_performed: `false`",
		"surfaces_with_risk_findings: `0`",
		"model_risk_findings: `0`",
		"high_risk_findings: `0`",
		"warning_risk_findings: `0`",
		"info_risk_findings: `0`",
		"raw_model_config_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_provider_error_bodies_included: `false`",
		"credential_values_included: `false`",
		"llm_e2e_required_after_model_risk_change: `true`",
		"### Provider Risk Card",
		"kind=`provider` provider=`github-models` model=`openai/gpt-5-nano` endpoint_host=`models.github.ai`",
		"### Fallback Risk Card",
		"kind=`fallback` fallback_models=`openai/gpt-4.1-nano`",
		"### Retry Risk Card",
		"kind=`retry` request_timeout_seconds=`75` retry_max_attempts=`6`",
		"### Config Model Risk Card",
		"kind=`model-config` path=`.gitclaw/config.yml` present=`true`",
		"risk_findings=`0`",
		"risk_codes=`none`",
		"### Current Model Request Risk Card",
		"scope=`local-cli` current_issue_model_request=`false`",
		"### Risk Findings",
		"- none",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("models risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"repository:", "issue:", "MODEL_RISK_SAFE_TOKEN", "trigger:", "base_url:", "openai/gpt-4.1-nano\n"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("models risk output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestRenderModelRiskReportFlagsUnsafeConfigWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/config.yml", `model:
  provider: openai-compatible
  model: custom-model
  fallbacks:
    - custom-model
  base_url: http://example.test/v1/chat/completions
  max_prompt_bytes: 0
  max_output_tokens: 0
  api_key: MODEL_RISK_CONFIG_SECRET
`)
	cfg := DefaultConfig()
	cfg.Workdir = root
	cfg.ModelProvider = "openai-compatible"
	cfg.Model = "custom-model"
	cfg.ModelFallbacks = []string{"custom-model"}
	cfg.LLMBaseURL = "http://example.test/v1/chat/completions"
	cfg.MaxPromptBytes = 0
	cfg.MaxOutputTokens = 0

	output := RenderModelRiskCLIReport(cfg)
	for _, want := range []string{
		"GitClaw Model Risk Report",
		"model_risk_status: `high`",
		"provider: `openai-compatible`",
		"model: `custom-model`",
		"endpoint_host: `example.test`",
		"github_models_endpoint: `false`",
		"primary_model_small_default: `false`",
		"primary_model_known_github_catalog_entry: `false`",
		"surfaces_with_risk_findings: `3`",
		"model_risk_findings: `9`",
		"high_risk_findings: `4`",
		"warning_risk_findings: `3`",
		"info_risk_findings: `2`",
		"code=`credential_material_in_model_config`",
		"code=`fallback_model_duplicates_primary`",
		"code=`insecure_model_endpoint`",
		"code=`max_output_tokens_not_positive`",
		"code=`max_prompt_bytes_not_positive`",
		"code=`model_id_unqualified`",
		"code=`non_github_models_endpoint`",
		"code=`non_github_models_provider`",
		"code=`primary_model_not_small_default`",
		"risk_max_severity=`high`",
		"line_sha256_12=",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("models risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"MODEL_RISK_CONFIG_SECRET", "api_key:", "base_url: http://example.test", "max_prompt_bytes: 0"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("models risk output leaked body text %q:\n%s", notWant, output)
		}
	}
}

func TestRenderModelReportRoutesRiskAuditWithoutBodies(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 161,
			"title": "@gitclaw /models risk",
			"body": "Hidden model risk body token: MODEL_RISK_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.ModelFallbacks = []string{"openai/gpt-4.1-nano"}
	body := RenderModelReport(ev, cfg)
	for _, want := range []string{
		"GitClaw Model Risk Report",
		"repository: `owner/repo`",
		"issue: `#161`",
		"model_risk_status: `ok`",
		"fallback_models: `openai/gpt-4.1-nano`",
		"issue_title_sha256_12:",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("model risk report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "MODEL_RISK_BODY_SECRET") {
		t.Fatalf("model risk report leaked body token:\n%s", body)
	}
}

func TestHandleModelRiskCommandPostsReportWithoutLLM(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "MODEL_RISK_HANDLER_TOKEN")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITCLAW_LLM_API_KEY", "")
	t.Setenv("GITCLAW_LLM_BASE_URL", "")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 162,
			"title": "@gitclaw /model risk-audit",
			"body": "Hidden model risk handler token: MODEL_RISK_HANDLER_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.ModelFallbacks = []string{"openai/gpt-4.1-nano"}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{162: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic model risk command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Model Risk Report",
		"Generated without a model call",
		"model=\"gitclaw/models\"",
		"model_risk_status: `ok`",
		"verification_scope: `github_models_control_plane`",
		"provider: `github-models`",
		"model: `openai/gpt-5-nano`",
		"fallback_models: `openai/gpt-4.1-nano`",
		"model_catalog_probe_performed: `false`",
		"live_inference_probe_performed: `false`",
		"raw_model_config_bodies_included: `false`",
		"raw_provider_error_bodies_included: `false`",
		"llm_e2e_required_after_model_risk_change: `true`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("model risk handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"MODEL_RISK_HANDLER_BODY_SECRET", "MODEL_RISK_HANDLER_TOKEN"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("model risk handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[162], "gitclaw:done") || hasLabel(github.IssueLabels[162], "gitclaw:running") || hasLabel(github.IssueLabels[162], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[162])
	}
}

func writeSafeModelRiskConfig(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/config.yml", `trigger:
  label: gitclaw
  prefix: "@gitclaw"

model:
  provider: github-models
  model: openai/gpt-5-nano
  fallbacks:
    - openai/gpt-4.1-nano
  base_url: https://models.github.ai/inference/chat/completions
  max_prompt_bytes: 60000
  max_output_tokens: 4000
  max_transcript_messages: 40
  max_transcript_message_bytes: 8000
`)
}
