package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writeToolDeferPlanFixture(t *testing.T, dir string) {
	t.Helper()
	writeTestFile(t, dir, ".gitclaw/TOOLS.md", "TOOL_DEFER_PLAN_GUIDANCE_SECRET: keep tool search bounded.\n")
	writeTestFile(t, dir, ".gitclaw/toolsets/repo-read.yaml", `name: repo-read
description: Read-only repository context tools for ordinary issue answers.
mode: read-only
tools:
  - gitclaw.list_files
  - gitclaw.search_files
  - gitclaw.read_file
  - gitclaw.skill_index
  - gitclaw.policy
instruction: |
  Prefer bounded repository search. TOOL_DEFER_PLAN_TOOLSET_SECRET.
`)
	writeTestFile(t, dir, ".gitclaw/mcp/github-read.yaml", `name: github-read
description: Metadata-only placeholder for a future reviewed GitHub MCP read surface.
transport: stdio
source: github-mcp-read
activation: metadata-only
tool_allowlist:
  - issues.read
  - contents.read
  - pull_requests.read
tool_denylist:
  - contents.write
  - actions.write
requires_secrets:
  - GITHUB_TOKEN
resources_enabled: false
prompts_enabled: false
`)
	writeTestFile(t, dir, "docs/search-fixture.md", "defer plan unique search fixture phrase => GITCLAW_DEFER_PLAN_CONTEXT_V1\n")
}

func TestRenderToolDeferPlanReportPlansProgressiveDisclosureWithoutRuntimeExposure(t *testing.T) {
	dir := t.TempDir()
	writeToolDeferPlanFixture(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	repoContext, err := LoadRepoContextWithConfig(dir, nil, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}

	report := RenderToolDeferPlanCLIReport(cfg, repoContext)
	for _, want := range []string{
		"GitClaw Tool Defer Plan Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"tool_defer_plan_status: `ok`",
		"defer_mode: `auto`",
		"threshold_pct: `10`",
		"context_budget_bytes: `60000`",
		"threshold_bytes: `6000`",
		"estimated_direct_catalog_bytes: `1800`",
		"estimated_deferrable_catalog_bytes: `1440`",
		"estimated_total_catalog_bytes: `3240`",
		"estimated_bridge_catalog_bytes: `900`",
		"activation_decision: `direct`",
		"activation_reason: `below_threshold`",
		"direct_core_entries: `5`",
		"enabled_core_entries: `5`",
		"deferrable_candidate_entries: `4`",
		"toolset_catalog_entries: `1`",
		"mcp_catalog_entries: `3`",
		"planned_direct_entries: `9`",
		"planned_deferred_entries: `0`",
		"candidate_bridge_tools: `3`",
		"planned_bridge_tools: `0`",
		"toolsets_scanned: `1`",
		"mcp_specs_scanned: `1`",
		"model_callable_structured_tools: `false`",
		"tool_search_bridge_runtime_enabled: `false`",
		"dynamic_mcp_discovery_allowed: `false`",
		"mcp_server_launch_allowed: `false`",
		"toolset_activation_supported: `false`",
		"raw_tool_schemas_included: `false`",
		"raw_toolset_bodies_included: `false`",
		"raw_toolset_instructions_included: `false`",
		"raw_mcp_bodies_included: `false`",
		"raw_mcp_command_args_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"llm_e2e_required_after_tool_defer_plan_change: `true`",
		"tool_validation_status: `ok`",
		"high_risk_findings: `0`",
		"warning_risk_findings: `0`",
		"info_risk_findings: `3`",
		"kind=`builtin-contract` name=`gitclaw.search_files`",
		"direct_core=`true` deferrable_candidate=`false` planned_deferred=`false` enabled=`true`",
		"kind=`toolset-profile` name=`repo-read`",
		"source=`repo-reviewed-toolset` path=`.gitclaw/toolsets/repo-read.yaml`",
		"tool_refs=`gitclaw.list_files, gitclaw.policy, gitclaw.read_file, gitclaw.search_files,",
		"kind=`mcp-tool` name=`github-read/contents.read`",
		"kind=`mcp-tool` name=`github-read/issues.read`",
		"kind=`mcp-tool` name=`github-read/pull_requests.read`",
		"reason=`mcp_tool_allowlist_ref`",
		"risk_codes=`none`",
		"code=`hermes_progressive_disclosure_threshold_evaluated`",
		"code=`structured_model_tools_disabled`",
		"code=`mcp_runtime_disabled`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("tool defer plan report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{
		"TOOL_DEFER_PLAN_GUIDANCE_SECRET",
		"TOOL_DEFER_PLAN_TOOLSET_SECRET",
		"GITCLAW_DEFER_PLAN_CONTEXT_V1",
		"defer plan unique search fixture phrase",
		"Prefer bounded repository search",
		"contents.write",
		"actions.write",
	} {
		if strings.Contains(report, leaked) {
			t.Fatalf("tool defer plan report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestToolsDeferPlanCommandReportsWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeToolDeferPlanFixture(t, dir)
	t.Setenv("GITCLAW_WORKDIR", dir)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"tools", "defer-plan"}); err != nil {
			t.Fatalf("tools defer-plan returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Tool Defer Plan Report",
		"scope: `local-cli`",
		"activation_decision: `direct`",
		"deferrable_candidate_entries: `4`",
		"planned_bridge_tools: `0`",
		"model_callable_structured_tools: `false`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("tools defer-plan output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"TOOL_DEFER_PLAN_GUIDANCE_SECRET", "TOOL_DEFER_PLAN_TOOLSET_SECRET", "GITCLAW_DEFER_PLAN_CONTEXT_V1"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("tools defer-plan output leaked %q:\n%s", leaked, output)
		}
	}
}

func TestRenderToolsReportRoutesDeferPlanWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeToolDeferPlanFixture(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 127,
			"title": "@gitclaw /tools defer-plan",
			"body": "Hidden tool defer-plan route token: TOOL_DEFER_PLAN_ROUTE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	repoContext, err := LoadRepoContextWithConfig(dir, []TranscriptMessage{{Role: "user", Body: ev.Issue.Body}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}

	report := RenderToolsReport(ev, cfg, repoContext)
	for _, want := range []string{
		"GitClaw Tool Defer Plan Report",
		"repository: `owner/repo`",
		"issue: `#127`",
		"tool_defer_plan_status: `ok`",
		"issue_title_sha256_12:",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("tools defer-plan routed report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"TOOL_DEFER_PLAN_ROUTE_SECRET", "TOOL_DEFER_PLAN_GUIDANCE_SECRET", "TOOL_DEFER_PLAN_TOOLSET_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("tools defer-plan routed report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestHandleToolsDeferPlanCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeToolDeferPlanFixture(t, dir)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 128,
			"title": "@gitclaw /tools defer-plan",
			"body": "Hidden tool defer-plan handler token: TOOL_DEFER_PLAN_HANDLER_SECRET.",
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
	cfg.Workdir = dir
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{128: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic tool defer-plan report", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Tool Defer Plan Report",
		"Generated without a model call",
		"model=\"gitclaw/tools\"",
		"repository: `owner/repo`",
		"issue: `#128`",
		"activation_decision: `direct`",
		"deferrable_candidate_entries: `4`",
		"model_callable_structured_tools: `false`",
		"llm_e2e_required_after_tool_defer_plan_change: `true`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("tools defer-plan handler report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"TOOL_DEFER_PLAN_HANDLER_SECRET", "TOOL_DEFER_PLAN_GUIDANCE_SECRET", "TOOL_DEFER_PLAN_TOOLSET_SECRET", "GITCLAW_DEFER_PLAN_CONTEXT_V1"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("tools defer-plan handler report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[128], "gitclaw:done") || hasLabel(github.IssueLabels[128], "gitclaw:running") || hasLabel(github.IssueLabels[128], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[128])
	}
}
