package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestToolApprovalPlanCommandReportsApprovalBoundaryWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "TOOL_APPROVAL_PLAN_GUIDANCE_SECRET: read-only tools only.")
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"tools", "approval-plan", "search_files"}); err != nil {
			t.Fatalf("tools approval-plan returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Tool Approval Plan Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"tool_approval_plan_status: `ok`",
		"requested_tool_sha256_12:",
		"normalized_tool: `gitclaw.search_files`",
		"matched_tools: `1`",
		"tool_enabled: `true`",
		"disabled_by_config: `false`",
		"blocked_by_allowlist: `false`",
		"tool_mode: `read-only`",
		"tool_trigger: `explicit quoted phrase or identifier`",
		"mutating_contract: `false`",
		"approval_required: `false`",
		"approval_decision: `no_approval_required_read_only`",
		"approval_store: `github-issue-labels`",
		"approval_scope: `per-issue`",
		"approval_label: `gitclaw:approved`",
		"needs_human_label: `gitclaw:needs-human`",
		"write_requested_label: `gitclaw:write-requested`",
		"approval_timeout_policy: `not_applicable_no_exec_tool`",
		"run_allowed_now: `true`",
		"run_mode: `read-only`",
		"write_actions_enabled: `false`",
		"model_call_required: `false`",
		"model_callable_structured_tools: `false`",
		"shell_execution_allowed: `false`",
		"network_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"raw_tool_name_included: `false`",
		"raw_inputs_included: `false`",
		"raw_outputs_included: `false`",
		"raw_approval_payloads_included: `false`",
		"tool_validation_status: `ok`",
		"llm_e2e_required_after_tool_approval_plan_change: `true`",
		"### Approval Gates",
		"gate=`tool_contract` status=`matched` matched_tools=`1`",
		"gate=`config_enabled` status=`passed` disabled_by_config=`false`",
		"gate=`allowlist` status=`passed` blocked_by_allowlist=`false`",
		"gate=`tool_mode` status=`read_only_or_metadata_only` mutating_contract=`false`",
		"gate=`approval_label` status=`not_required` label=`gitclaw:approved`",
		"gate=`write_mode` status=`blocked` detail=`read_only_v1`",
		"gate=`structured_model_tools` status=`disabled`",
		"gate=`shell_exec` status=`disabled`",
		"gate=`repository_mutation` status=`disabled`",
		"### Contract",
		"name=`gitclaw.search_files` source=`builtin-gitclaw` enabled=`true`",
		"### Findings",
		"code=`openclaw_exec_approval_boundary_modeled`",
		"code=`hermes_tool_authorization_boundary_modeled`",
		"code=`github_issue_approval_store_modeled`",
		"code=`read_only_or_metadata_only_no_approval_required`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("tool approval-plan output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"TOOL_APPROVAL_PLAN_GUIDANCE_SECRET"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("tool approval-plan output leaked body/input token %q:\n%s", leaked, output)
		}
	}
}

func TestRenderToolsReportRoutesApprovalPlanWithoutBodies(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 191,
			"title": "@gitclaw /tools approval-plan e2e",
			"body": "@gitclaw /tools approval-plan read_file\nHidden tools approval route token: TOOL_APPROVAL_ROUTE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	body := RenderToolsReport(ev, DefaultConfig(), RepoContext{Documents: []ContextDocument{{Path: ".gitclaw/TOOLS.md", Body: "TOOL_APPROVAL_ROUTE_GUIDANCE_SECRET"}}})
	for _, want := range []string{
		"GitClaw Tool Approval Plan Report",
		"repository: `owner/repo`",
		"issue: `#191`",
		"tool_approval_plan_status: `ok`",
		"normalized_tool: `gitclaw.read_file`",
		"approval_decision: `no_approval_required_read_only`",
		"issue_title_sha256_12:",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("tool approval-plan routed report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"TOOL_APPROVAL_ROUTE_SECRET", "TOOL_APPROVAL_ROUTE_GUIDANCE_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("tool approval-plan routed report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestHandleToolsApprovalPlanCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "TOOLS_APPROVAL_HANDLER_SECRET: read-only tools only.")
	writeTestFile(t, root, "docs/search-fixture.md", "tool approval unique search fixture phrase => GITCLAW_TOOL_APPROVAL_CONTEXT_V1\n")

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 192,
			"title": "@gitclaw /tools approval-plan search_files",
			"body": "Search for \"tool approval unique search fixture phrase\". Hidden tools approval body token: TOOLS_APPROVAL_HANDLER_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{192: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = root
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic tools approval-plan command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Tool Approval Plan Report",
		"Generated without a model call",
		"model=\"gitclaw/tools\"",
		"repository: `owner/repo`",
		"issue: `#192`",
		"tool_approval_plan_status: `ok`",
		"normalized_tool: `gitclaw.search_files`",
		"matched_tools: `1`",
		"active_outputs_for_tool: `1`",
		"approval_required: `false`",
		"approval_decision: `no_approval_required_read_only`",
		"run_allowed_now: `true`",
		"model_callable_structured_tools: `false`",
		"shell_execution_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"raw_inputs_included: `false`",
		"raw_outputs_included: `false`",
		"tool_validation_status: `ok`",
		"llm_e2e_required_after_tool_approval_plan_change: `true`",
		"### Approval Gates",
		"gate=`approval_label` status=`not_required` label=`gitclaw:approved`",
		"### Active Outputs For Tool",
		"contract_known=`true`",
		"input_sha256_12=",
		"output_sha256_12=",
		"### Findings",
		"code=`read_only_or_metadata_only_no_approval_required`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("tools approval-plan report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"TOOLS_APPROVAL_HANDLER_SECRET", "TOOLS_APPROVAL_HANDLER_BODY_SECRET", "GITCLAW_TOOL_APPROVAL_CONTEXT_V1", "tool approval unique search fixture phrase"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("tools approval-plan report leaked body/input/output token %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[192], "gitclaw:done") || hasLabel(github.IssueLabels[192], "gitclaw:running") || hasLabel(github.IssueLabels[192], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[192])
	}
}
