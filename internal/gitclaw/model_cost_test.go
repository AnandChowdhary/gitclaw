package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestModelsCostCommandReportsReviewedPricingSurfaceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeSafeModelRiskConfig(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)
	t.Setenv("GITHUB_TOKEN", "MODEL_COST_SAFE_TOKEN")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"models", "cost"}); err != nil {
			t.Fatalf("models cost returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Model Cost Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"model_cost_status: `warn`",
		"verification_scope: `github_models_direct_cost_catalog`",
		"provider: `github-models`",
		"model: `openai/gpt-5-nano`",
		"fallback_models: `openai/gpt-4.1-nano`",
		"endpoint_host: `models.github.ai`",
		"token_source: `GITHUB_TOKEN`",
		"pricing_source: `github_models_direct_costs_snapshot`",
		"pricing_source_url: `https://docs.github.com/en/billing/reference/costs-for-github-models`",
		"pricing_snapshot_date: `2026-05-31`",
		"token_unit_price_usd: `0.00001`",
		"catalog_entries: `15`",
		"current_model_catalog_entry_present: `false`",
		"current_model_cost_estimation_supported: `false`",
		"projected_usd: `unavailable`",
		"recorded_usage_cost_estimation_supported: `false`",
		"usage_bearing_assistant_turns: `0`",
		"costed_usage_turns: `0`",
		"uncosted_usage_turns: `0`",
		"recorded_estimated_usd: `unavailable`",
		"billing_api_probe_performed: `false`",
		"live_inference_probe_performed: `false`",
		"billing_account_state_known: `false`",
		"paid_usage_opt_in_state_known: `false`",
		"github_budget_state_known: `false`",
		"raw_provider_usage_included: `false`",
		"raw_provider_response_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"credential_values_included: `false`",
		"llm_e2e_required_after_model_cost_change: `true`",
		"### Cost Cards",
		"kind=`current-model-cost` model=`openai/gpt-5-nano` catalog_entry_present=`false`",
		"kind=`recorded-usage-cost` usage_bearing_assistant_turns=`0`",
		"### Usage Cost Lines",
		"- none",
		"code=`github_models_token_unit_pricing_modeled`",
		"code=`openclaw_usage_cost_surface_modeled`",
		"code=`hermes_api_token_count_boundary_modeled`",
		"code=`billing_api_not_queried`",
		"code=`current_model_multiplier_unknown`",
		"code=`no_usage_markers_seen`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("models cost output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"repository:", "issue:", "MODEL_COST_SAFE_TOKEN", "base_url:"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("models cost output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestRenderModelCostReportEstimatesKnownUsageMarkersWithoutBodies(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 181,
			"title": "@gitclaw /models cost",
			"body": "Hidden model cost body token: MODEL_COST_BODY_SECRET.",
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
	cfg.Model = "openai/gpt-4o"
	comment := Comment{
		ID:                91,
		AuthorAssociation: "NONE",
		User:              User{Login: "github-actions[bot]", Type: "Bot"},
		Body: RenderAssistantComment(withPromptProvenance(Marker{
			Model: "openai/gpt-4o",
			Usage: LLMUsage{
				Present:          true,
				PromptTokens:     1000000,
				CompletionTokens: 1000000,
				TotalTokens:      2000000,
			},
		}, RepoContext{ToolOutputs: []ToolOutput{{Name: "gitclaw.search_files", Output: "MODEL_COST_COMMENT_SECRET"}}}), "MODEL_COST_ASSISTANT_SECRET"),
	}
	transcript := BuildTranscript(ev, []Comment{comment})
	body := RenderModelCostReport(ev, cfg, []Comment{comment}, transcript, RepoContext{})
	for _, want := range []string{
		"GitClaw Model Cost Report",
		"repository: `owner/repo`",
		"issue: `#181`",
		"model_cost_status: `ok`",
		"current_model_catalog_entry_present: `true`",
		"current_model_catalog_name: `OpenAI GPT-4o`",
		"current_model_input_multiplier: `0.25`",
		"current_model_cached_input_multiplier: `0.125`",
		"current_model_cached_input_multiplier_set: `true`",
		"current_model_output_multiplier: `1`",
		"current_model_cost_estimation_supported: `true`",
		"recorded_usage_cost_estimation_supported: `true`",
		"assistant_turn_markers: `1`",
		"model_backed_assistant_turns: `1`",
		"deterministic_assistant_turns: `0`",
		"usage_bearing_assistant_turns: `1`",
		"costed_usage_turns: `1`",
		"uncosted_usage_turns: `0`",
		"model_names: `openai/gpt-4o`",
		"uncosted_model_names: `none`",
		"recorded_prompt_tokens: `1000000`",
		"recorded_completion_tokens: `1000000`",
		"recorded_total_tokens: `2000000`",
		"recorded_token_units: `1250000`",
		"recorded_estimated_usd: `12.5`",
		"kind=`recorded-usage-cost` usage_bearing_assistant_turns=`1` costed_usage_turns=`1`",
		"source=`comment:91` model=`openai/gpt-4o` prompt_tokens=`1000000` completion_tokens=`1000000` total_tokens=`2000000`",
		"catalog_entry_present=`true` catalog_model_name=`OpenAI GPT-4o` token_units=`1250000` estimated_usd=`12.5`",
		"code=`recorded_usage_cost_estimated`",
		"issue_title_sha256_12:",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("model cost report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"MODEL_COST_BODY_SECRET", "MODEL_COST_ASSISTANT_SECRET", "MODEL_COST_COMMENT_SECRET", "current_model_multiplier_unknown", "no_usage_markers_seen"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("model cost report leaked or misclassified %q:\n%s", notWant, body)
		}
	}
}

func TestRenderModelReportRoutesCostWithoutBodies(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 182,
			"title": "@gitclaw /model billing",
			"body": "Hidden model cost route token: MODEL_COST_ROUTE_SECRET.",
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
		"GitClaw Model Cost Report",
		"repository: `owner/repo`",
		"issue: `#182`",
		"pricing_source: `github_models_direct_costs_snapshot`",
		"billing_api_probe_performed: `false`",
		"issue_title_sha256_12:",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("model cost routed report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "MODEL_COST_ROUTE_SECRET") {
		t.Fatalf("model cost routed report leaked body token:\n%s", body)
	}
}

func TestHandleModelCostCommandPostsReportWithoutLLM(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "MODEL_COST_HANDLER_TOKEN")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 183,
			"title": "@gitclaw /models cost",
			"body": "Hidden model cost handler token: MODEL_COST_HANDLER_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{183: nil}}
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
		"GitClaw Model Cost Report",
		"model_cost_status: `warn`",
		"pricing_source_url: `https://docs.github.com/en/billing/reference/costs-for-github-models`",
		"raw_issue_bodies_included: `false`",
	} {
		if !strings.Contains(comment, want) {
			t.Fatalf("model cost handler comment missing %q:\n%s", want, comment)
		}
	}
	if strings.Contains(comment, "MODEL_COST_HANDLER_BODY_SECRET") || strings.Contains(comment, "MODEL_COST_HANDLER_TOKEN") {
		t.Fatalf("model cost handler leaked secret:\n%s", comment)
	}
}
