package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestModelsCatalogCommandReportsReviewedSnapshotWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeSafeModelRiskConfig(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)
	t.Setenv("GITHUB_TOKEN", "MODEL_CATALOG_SAFE_TOKEN")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"models", "catalog"}); err != nil {
			t.Fatalf("models catalog returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Model Catalog Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"model_catalog_status: `ok`",
		"provider: `github-models`",
		"model: `openai/gpt-5-nano`",
		"fallback_models: `openai/gpt-4.1-nano`",
		"default_model_policy: `smallest-openai-gpt5-github-models-catalog-model`",
		"catalog_source: `reviewed-github-models-catalog-snapshot`",
		"catalog_source_url: `https://docs.github.com/en/rest/models/catalog`",
		"inference_source_url: `https://docs.github.com/en/rest/models/inference`",
		"catalog_api_version: `2026-03-10`",
		"catalog_endpoint_host: `models.github.ai`",
		"endpoint_host: `models.github.ai`",
		"token_source: `GITHUB_TOKEN`",
		"catalog_snapshot_date: `2026-06-01`",
		"reviewed_catalog_entries: `9`",
		"reviewed_openai_entries: `9`",
		"reviewed_gpt5_entries: `4`",
		"configured_model_catalog_entry_present: `true`",
		"fallback_models_configured: `1`",
		"fallback_models_catalog_entries: `1`",
		"default_candidate: `openai/gpt-5-nano`",
		"default_candidate_catalog_entry_present: `true`",
		"configured_model_matches_default_candidate: `true`",
		"gpt_5_4_mini_catalog_entry_present: `false`",
		"newer_small_model_candidate_present: `false`",
		"model_catalog_probe_performed: `false`",
		"live_inference_probe_performed: `false`",
		"raw_catalog_response_included: `false`",
		"raw_provider_response_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"credential_values_included: `false`",
		"llm_e2e_required_after_model_catalog_change: `true`",
		"### Catalog Cards",
		"model_id=`openai/gpt-5-nano`",
		"name_sha256_12=",
		"publisher=`OpenAI`",
		"family=`gpt-5`",
		"size_class=`nano`",
		"default_candidate=`true`",
		"model_id=`openai/gpt-4.1-nano`",
		"### Catalog Gates",
		"configured_model_gate=`pass`",
		"fallback_model_gate=`pass`",
		"default_candidate_gate=`pass`",
		"gpt_5_4_mini_gate=`not-present`",
		"live_probe_gate=`disabled-for-deterministic-report`",
		"raw_body_gate=`ids-metadata-and-hashes-only`",
		"### Findings",
		"code=`gpt_5_4_mini_not_in_reviewed_catalog`",
		"code=`live_catalog_probe_not_performed`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("models catalog output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"repository:", "issue:", "MODEL_CATALOG_SAFE_TOKEN", "api_key:", "base_url:"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("models catalog output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestRenderModelReportRoutesCatalogWithoutBodies(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ModelFallbacks = []string{"openai/gpt-4.1-nano"}
	ev := Event{
		Kind:      "issue_comment",
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 190, Title: "@gitclaw /models catalog MODEL_CATALOG_TITLE_SECRET", Body: "MODEL_CATALOG_ISSUE_SECRET"},
		Comment:   &Comment{ID: 55, Body: "@gitclaw /models catalog\nMODEL_CATALOG_COMMENT_SECRET", User: User{Login: "alice"}, AuthorAssociation: "MEMBER"},
	}
	body := RenderModelReport(ev, cfg)
	for _, want := range []string{
		"GitClaw Model Catalog Report",
		"repository: `owner/repo`",
		"issue: `#190`",
		"event_kind: `issue_comment`",
		"event_name: `issue_comment`",
		"model_catalog_status: `ok`",
		"configured_model_matches_default_candidate: `true`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"issue_title_sha256_12:",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("models catalog issue output missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"MODEL_CATALOG_TITLE_SECRET", "MODEL_CATALOG_ISSUE_SECRET", "MODEL_CATALOG_COMMENT_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("models catalog issue output leaked %q:\n%s", leaked, body)
		}
	}
}

func TestHandleModelCatalogCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITCLAW_WORKDIR", dir)
	writeSafeModelRiskConfig(t, dir)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 191,
			"title": "@gitclaw /models catalog",
			"body": "@gitclaw /models catalog\nHidden: MODEL_CATALOG_HANDLER_ISSUE_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{191: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = dir
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic model catalog command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"model=\"gitclaw/models\"",
		"GitClaw Model Catalog Report",
		"model_catalog_status: `ok`",
		"repository: `owner/repo`",
		"issue: `#191`",
		"configured_model_catalog_entry_present: `true`",
		"configured_model_matches_default_candidate: `true`",
		"llm_e2e_required_after_model_catalog_change: `true`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("models catalog handler output missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "MODEL_CATALOG_HANDLER_ISSUE_SECRET") {
		t.Fatalf("models catalog handler output leaked issue body:\n%s", body)
	}
	if !hasLabel(github.IssueLabels[191], "gitclaw:done") || hasLabel(github.IssueLabels[191], "gitclaw:running") || hasLabel(github.IssueLabels[191], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[191])
	}
}
