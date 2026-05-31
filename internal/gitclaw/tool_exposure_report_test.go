package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writeToolExposureFixture(t *testing.T, dir string) {
	t.Helper()
	writeTestFile(t, dir, ".gitclaw/TOOLS.md", "Use deterministic read-only GitClaw tools as bounded prompt context.\n")
	writeTestFile(t, dir, "docs/search.md", "bounded repository search fixture phrase -> GITCLAW_SEARCH_CONTEXT_V1\n")
}

func TestRenderToolExposureReportAuditsPromptVisibleBoundary(t *testing.T) {
	dir := t.TempDir()
	writeToolExposureFixture(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	repoContext, err := LoadRepoContextWithConfig(dir, nil, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}

	report := RenderToolExposureCLIReport(repoContext)
	for _, want := range []string{
		"GitClaw Tool Exposure Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"tool_exposure_status: `ok`",
		"exposure_strategy: `static-pre-model-context`",
		"bridge_strategy: `not_enabled_in_v1`",
		"available_tools: `5`",
		"enabled_tool_contracts: `5`",
		"disabled_tool_contracts: `0`",
		"allowlist_blocked_tool_contracts: `0`",
		"explicit_allowlist_configured: `false`",
		"exposed_read_only_contracts: `3`",
		"exposed_metadata_only_contracts: `2`",
		"mutating_tool_contracts: `0`",
		"active_tool_outputs: `1`",
		"known_active_tool_outputs: `1`",
		"unknown_active_tool_outputs: `0`",
		"prompt_visible_tool_outputs: `1`",
		"model_callable_structured_tools: `false`",
		"deferred_tool_schemas: `0`",
		"tool_search_bridge_tools: `0`",
		"fail_closed_required: `false`",
		"tool_validation_status: `ok`",
		"tool_validation_errors: `0`",
		"tool_validation_warnings: `0`",
		"shell_execution_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"network_tool_execution_allowed: `false`",
		"raw_tool_schemas_included: `false`",
		"raw_tool_inputs_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_issue_bodies_included: `false`",
		"llm_e2e_required_after_tool_exposure_change: `true`",
		"tool_name=`gitclaw.list_files`",
		"enabled=`true`",
		"exposed_for_prompt=`true`",
		"active_outputs=`1`",
		"tool_name=`gitclaw.search_files`",
		"tool_name=`gitclaw.policy`",
		"exposure_codes=`none`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("tool exposure report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"GITCLAW_SEARCH_CONTEXT_V1", "bounded repository search fixture phrase", "Use deterministic read-only GitClaw tools"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("tool exposure report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestRenderToolExposureRiskReportFlagsFailClosedAllowlist(t *testing.T) {
	dir := t.TempDir()
	writeToolExposureFixture(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	cfg.AllowedTools = map[string]bool{"gitclaw.read_file": true}
	cfg.DisabledTools = map[string]bool{"gitclaw.read_file": true}
	repoContext, err := LoadRepoContextWithConfig(dir, nil, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}

	report := RenderToolExposureRiskCLIReport(repoContext)
	for _, want := range []string{
		"GitClaw Tool Exposure Risk Report",
		"tool_exposure_status: `high`",
		"enabled_tool_contracts: `0`",
		"disabled_tool_contracts: `1`",
		"allowlist_blocked_tool_contracts: `4`",
		"explicit_allowlist_configured: `true`",
		"allowed_tool_names: `gitclaw.read_file`",
		"disabled_tool_names: `gitclaw.read_file`",
		"active_tool_outputs: `0`",
		"fail_closed_required: `true`",
		"high_risk_findings: `1`",
		"warning_risk_findings: `1`",
		"info_risk_findings: `3`",
		"tool_name=`gitclaw.read_file`",
		"enabled=`false` disabled_by_config=`true` blocked_by_allowlist=`false`",
		"tool_name=`gitclaw.search_files`",
		"blocked_by_allowlist=`true`",
		"code=`explicit_allowlist_resolved_zero`",
		"code=`no_enabled_tool_contracts`",
		"code=`hermes_tool_search_bridge_not_enabled`",
		"code=`static_pre_model_tool_context`",
		"code=`structured_model_tools_disabled`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("tool exposure risk report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"GITCLAW_SEARCH_CONTEXT_V1", "bounded repository search fixture phrase", "Use deterministic read-only GitClaw tools"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("tool exposure risk report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestToolsExposureCommandsReportBoundary(t *testing.T) {
	dir := t.TempDir()
	writeToolExposureFixture(t, dir)
	t.Setenv("GITCLAW_WORKDIR", dir)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"tools", "exposure"}); err != nil {
			t.Fatalf("tools exposure returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Tool Exposure Report", "scope: `local-cli`", "tool_exposure_status: `ok`", "enabled_tool_contracts: `5`", "model_callable_structured_tools: `false`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("tools exposure output missing %q:\n%s", want, output)
		}
	}

	riskOutput := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"tools", "exposure", "risk"}); err != nil {
			t.Fatalf("tools exposure risk returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Tool Exposure Risk Report", "tool_exposure_status: `ok`", "### Exposure Findings", "code=`static_pre_model_tool_context`"} {
		if !strings.Contains(riskOutput, want) {
			t.Fatalf("tools exposure risk output missing %q:\n%s", want, riskOutput)
		}
	}
	if strings.Contains(output, "GITCLAW_SEARCH_CONTEXT_V1") || strings.Contains(riskOutput, "GITCLAW_SEARCH_CONTEXT_V1") {
		t.Fatalf("tools exposure CLI leaked fixture token:\n%s\n%s", output, riskOutput)
	}
}

func TestRenderToolsReportRoutesExposureRiskWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeToolExposureFixture(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 125,
			"title": "@gitclaw /tools exposure risk",
			"body": "Hidden tool exposure route token: TOOL_EXPOSURE_ROUTE_SECRET.",
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
	for _, want := range []string{"GitClaw Tool Exposure Risk Report", "repository: `owner/repo`", "issue: `#125`", "tool_exposure_status: `ok`", "issue_title_sha256_12:"} {
		if !strings.Contains(report, want) {
			t.Fatalf("tools exposure routed report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"TOOL_EXPOSURE_ROUTE_SECRET", "Use deterministic read-only GitClaw tools"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("tools exposure routed report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestHandleToolsExposureRiskCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeToolExposureFixture(t, dir)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 126,
			"title": "@gitclaw /tools exposure risk",
			"body": "Hidden tool exposure handler token: TOOL_EXPOSURE_HANDLER_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{126: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic tool exposure report", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Tool Exposure Risk Report", "Generated without a model call", "model=\"gitclaw/tools\"", "tool_exposure_status: `ok`", "model_callable_structured_tools: `false`", "llm_e2e_required_after_tool_exposure_change: `true`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("tools exposure handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"TOOL_EXPOSURE_HANDLER_SECRET", "Use deterministic read-only GitClaw tools"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("tools exposure handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[126], "gitclaw:done") || hasLabel(github.IssueLabels[126], "gitclaw:running") || hasLabel(github.IssueLabels[126], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[126])
	}
}
