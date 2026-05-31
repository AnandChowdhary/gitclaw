package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestModelsUsageCommandReportsTelemetrySurfaceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeSafeModelRiskConfig(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)
	t.Setenv("GITHUB_TOKEN", "MODEL_USAGE_SAFE_TOKEN")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"models", "usage"}); err != nil {
			t.Fatalf("models usage returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Model Usage Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"model_usage_status: `warn`",
		"verification_scope: `github_models_usage_surface`",
		"provider: `github-models`",
		"model: `openai/gpt-5-nano`",
		"fallback_models: `openai/gpt-4.1-nano`",
		"default_model_policy: `smallest-openai-github-models-catalog-model`",
		"catalog_endpoint_host: `models.github.ai`",
		"endpoint_host: `models.github.ai`",
		"token_source: `GITHUB_TOKEN`",
		"output_token_parameter: `max_completion_tokens`",
		"github_models_endpoint: `true`",
		"github_actions_token_supported: `true`",
		"usage_response_parsing_enabled: `true`",
		"usage_marker_persistence_enabled: `true`",
		"live_inference_probe_performed: `false`",
		"billing_api_probe_performed: `false`",
		"cost_estimation_supported: `false`",
		"cost_estimation_reason: `pricing_catalog_not_configured`",
		"recorded_prompt_tokens: `0`",
		"recorded_completion_tokens: `0`",
		"recorded_total_tokens: `0`",
		"raw_provider_usage_included: `false`",
		"raw_provider_response_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"credential_values_included: `false`",
		"llm_e2e_required_after_model_usage_change: `true`",
		"### Usage Telemetry Cards",
		"kind=`provider` provider=`github-models` model=`openai/gpt-5-nano` endpoint_host=`models.github.ai`",
		"kind=`prompt-projection`",
		"kind=`session-usage` assistant_turn_markers=`0`",
		"kind=`latest-usage` present=`false`",
		"code=`openclaw_token_usage_surface_modeled`",
		"code=`hermes_api_token_counts_modeled`",
		"code=`github_models_actions_token_boundary_modeled`",
		"code=`usage_marker_persistence_enabled`",
		"code=`cost_estimation_disabled_until_pricing_config`",
		"code=`no_usage_markers_seen`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("models usage output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"repository:", "issue:", "MODEL_USAGE_SAFE_TOKEN", "base_url:"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("models usage output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestRenderModelUsageReportAggregatesUsageMarkersWithoutBodies(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 171,
			"title": "@gitclaw /models usage",
			"body": "Hidden model usage body token: MODEL_USAGE_BODY_SECRET.",
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
	comment := Comment{
		ID:                91,
		AuthorAssociation: "NONE",
		User:              User{Login: "github-actions[bot]", Type: "Bot"},
		Body: RenderAssistantComment(withPromptProvenance(Marker{
			Model: "openai/gpt-5-nano",
			Usage: LLMUsage{
				Present:          true,
				PromptTokens:     120,
				CompletionTokens: 30,
				TotalTokens:      150,
				CacheReadTokens:  80,
			},
		}, RepoContext{ToolOutputs: []ToolOutput{{Name: "gitclaw.search_files", Output: "MODEL_USAGE_COMMENT_SECRET"}}}), "MODEL_USAGE_ASSISTANT_SECRET"),
	}
	transcript := BuildTranscript(ev, []Comment{comment})
	body := RenderModelUsageReport(ev, cfg, []Comment{comment}, transcript, RepoContext{})
	for _, want := range []string{
		"GitClaw Model Usage Report",
		"repository: `owner/repo`",
		"issue: `#171`",
		"model_usage_status: `ok`",
		"assistant_turn_markers: `1`",
		"model_backed_assistant_turns: `1`",
		"deterministic_assistant_turns: `0`",
		"usage_bearing_assistant_turns: `1`",
		"model_names: `openai/gpt-5-nano`",
		"recorded_prompt_tokens: `120`",
		"recorded_completion_tokens: `30`",
		"recorded_total_tokens: `150`",
		"recorded_cache_read_tokens: `80`",
		"latest_usage_model: `openai/gpt-5-nano`",
		"latest_usage_total_tokens: `150`",
		"kind=`latest-usage` present=`true` model=`openai/gpt-5-nano` prompt_tokens=`120` completion_tokens=`30` total_tokens=`150` cache_read_tokens=`80`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("model usage report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"MODEL_USAGE_BODY_SECRET", "MODEL_USAGE_ASSISTANT_SECRET", "MODEL_USAGE_COMMENT_SECRET"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("model usage report leaked body text %q:\n%s", notWant, body)
		}
	}
}

func TestRenderModelReportRoutesUsageWithoutBodies(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 172,
			"title": "@gitclaw /model tokens",
			"body": "Hidden model usage route token: MODEL_USAGE_ROUTE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	body := RenderModelReport(ev, DefaultConfig())
	for _, want := range []string{
		"GitClaw Model Usage Report",
		"repository: `owner/repo`",
		"issue: `#172`",
		"usage_response_parsing_enabled: `true`",
		"issue_title_sha256_12:",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("model usage routed report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "MODEL_USAGE_ROUTE_SECRET") {
		t.Fatalf("model usage routed report leaked body token:\n%s", body)
	}
}

func TestHandleModelUsageCommandPostsReportWithoutLLM(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "MODEL_USAGE_HANDLER_TOKEN")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 173,
			"title": "@gitclaw /models usage",
			"body": "Hidden model usage handler token: MODEL_USAGE_HANDLER_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{173: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM calls = %d, want 0", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted comments = %d, want 1", len(github.Posted))
	}
	comment := github.Posted[0].Body
	for _, want := range []string{
		"model=\"gitclaw/models\"",
		"GitClaw Model Usage Report",
		"model_usage_status: `warn`",
		"raw_issue_bodies_included: `false`",
	} {
		if !strings.Contains(comment, want) {
			t.Fatalf("model usage handler comment missing %q:\n%s", want, comment)
		}
	}
	if strings.Contains(comment, "MODEL_USAGE_HANDLER_BODY_SECRET") || strings.Contains(comment, "MODEL_USAGE_HANDLER_TOKEN") {
		t.Fatalf("model usage handler leaked secret:\n%s", comment)
	}
}
